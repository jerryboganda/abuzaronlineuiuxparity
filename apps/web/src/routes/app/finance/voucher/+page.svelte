<script lang="ts">
  import { onMount } from 'svelte';
  import type { MasterRecord } from '@abuzar/contracts';
  import { AbuzarApi, ApiError } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';

  const api = new AbuzarApi();
  let categoryCode = 'receipt';
  let partyId = '';
  let amount = '';
  let description = '';
  let debitAccountId = '';
  let creditAccountId = '';
  let message = '';
  let error = '';
  let busy = false;
  let customers: MasterRecord[] = [];
  let suppliers: MasterRecord[] = [];
  let accounts: Array<{ id: string; code: string; name: string; category: string; active: boolean }> = [];

  $: parties = categoryCode === 'payment' ? suppliers : customers;
  $: if (categoryCode !== 'journal' && partyId && !parties.some((party) => party.id === partyId)) partyId = '';

  onMount(() => {
    void loadLookups();
  });

  async function loadLookups() {
    try {
      const [customerResult, supplierResult] = await Promise.all([
        api.masterRecords('customer'),
        api.masterRecords('supplier')
      ]);
      customers = customerResult.records.filter((record) => record.active);
      suppliers = supplierResult.records.filter((record) => record.active);
    } catch (cause) {
      error = cause instanceof ApiError ? cause.message : 'Party lookup failed.';
    }
    try {
      accounts = (await api.financeAccounts()).accounts.filter((account) => account.active);
    } catch {
      accounts = [];
    }
  }

  async function postVoucher() {
    error = '';
    message = '';
    busy = true;
    try {
      const payload: Record<string, unknown> = {
        categoryCode,
        amount,
        description,
        occurredAt: new Date().toISOString()
      };
      if (categoryCode === 'journal') {
        payload.lines = [
          { accountId: debitAccountId, debit: amount, credit: '0', memo: description },
          { accountId: creditAccountId, debit: '0', credit: amount, memo: description }
        ];
      } else {
        payload.partyId = partyId;
      }
      const result = await api.postVoucher(payload);
      message = result.message || `Posted ${result.kind}.`;
    } catch (cause) {
      error = cause instanceof ApiError ? cause.message : 'Voucher posting failed.';
    } finally {
      busy = false;
    }
  }
</script>

<section class="legacy-transaction-page">
  <LegacyMenuBar context="base" windowId="voucher" windowLabel="Vouchers" windowHref="/app/finance/voucher" />
  <div class="legacy-transaction-window">
    <h1>Accounting Vouchers</h1>
    {#if error}<p class="module-message error">{error}</p>{/if}
    {#if message}<p class="module-message">{message}</p>{/if}
    <form class="legacy-transaction-fields" onsubmit={(event) => { event.preventDefault(); void postVoucher(); }}>
      <label>Type
        <select bind:value={categoryCode} aria-label="Voucher type">
          <option value="receipt">Receipt</option>
          <option value="payment">Payment</option>
          <option value="journal">Journal</option>
        </select>
      </label>
      {#if categoryCode !== 'journal'}
        <label>{categoryCode === 'payment' ? 'Supplier' : 'Customer'}
          <select bind:value={partyId} aria-label={categoryCode === 'payment' ? 'Supplier' : 'Customer'} required>
            <option value="">Select {categoryCode === 'payment' ? 'supplier' : 'customer'}</option>
            {#each parties as party}
              <option value={party.id}>{party.code} — {party.name}</option>
            {/each}
          </select>
        </label>
      {:else}
        <label>Debit account
          <select bind:value={debitAccountId} aria-label="Debit account" required>
            <option value="">Select debit account</option>
            {#each accounts as account}
              <option value={account.id}>{account.code} — {account.name}</option>
            {/each}
          </select>
        </label>
        <label>Credit account
          <select bind:value={creditAccountId} aria-label="Credit account" required>
            <option value="">Select credit account</option>
            {#each accounts as account}
              <option value={account.id}>{account.code} — {account.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      <label>Amount
        <input bind:value={amount} aria-label="Voucher amount" required />
      </label>
      <label>Description
        <input bind:value={description} aria-label="Voucher description" />
      </label>
      <button type="submit" disabled={busy}>{busy ? 'Posting…' : 'Post voucher'}</button>
    </form>
  </div>
</section>
