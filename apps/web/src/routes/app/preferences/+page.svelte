<script lang="ts">
  import { onMount } from 'svelte';
  import { AbuzarApi, ApiError } from '$lib/api';

  type PreferenceRow = { caption: string; value: string; section?: boolean };
  const section = (caption: string): PreferenceRow => ({ caption, value: '', section: true });
  const item = (caption: string, value: string): PreferenceRow => ({ caption, value });
  const tabs = ['General', 'Sale', 'Sale Return', 'Purchase', 'Purchase Return', 'Report', 'BasicData', 'Quotation', 'Schedule', 'Adjustment', 'Purchase Order', 'Others', 'Point of Sale', 'Cashier Job Activity', 'Email', 'SMS', 'Dashboard'];

  const general: PreferenceRow[] = [
    section('Software Default Settings'),
    item('Inventory System:', 'Periodic Inventory System'), item('Inventory Movement Method:', 'Equal Priority / Shortest Expiry First'), item('Default Batch:', '.'), item('Enable Alias Name:', 'No'), item('Search Item Code if Alias Name Not Found:', 'No'), item('Default Expiry:', '2030-12-12 00:00:00'), item('Prompt Before Printing:', 'No'), item('Ask No. of copies in print dialog:', 'Yes'), item('Max. Allowed Days:', '30'), item('Business Short Name:', ''), item('Application Title:', 'ABUZAR V3 01.01.2025'), item('Allow Login A User Multiple Times:', 'Yes'), item('Auto Responsive Search With Alternate Alias Name:', 'No'),
    section('Item Search Window Column(s)'),
    item('Item Name:', 'Yes'), item('Alias Name:', 'No'), item('Local Item Name:', 'No'), item('Manufacturer:', 'Yes'), item('Sale Price:', 'Yes'), item('Packing:', 'No'), item('Pack Units:', 'Yes'), item('Stock:', 'Yes'), item('Pack Stock', 'NO'), item('Generic Item Name', 'No'), item('Packing Description:', 'No'), item('Item Location:', 'Yes'), item('Transit Stock:', 'No'), item('Item Alert:', 'No'),
    section('Group Customer Category Rights'),
    item('In Sale/Return:', 'No'), item('In Basic Data:', 'No'), item('In Accounts Module:', 'No'),
    section('Group Wise Supplier Category'),
    item('In Purchase/Purchase Return:', 'No'), item('In Purchase/Purchase Return Services:', 'No'), item('In Accounts:', 'No'), item('In Basic Data:', 'No'),
    section('Group Item Category Rights'),
    item('In Basic Data:', 'No'), item('In Sale/Return:', 'No'),
    section('Quick Preferred Search'),
    item('Enable Quick Search:', 'No'), item('Quick Search Type:', 'X'),
    section('Backup Preferences'),
    item('Check Manual Backup Health At StartUp:', 'Yes'),
    section('Data Carry DB'),
    item('Age of Item Changes For Data Carry (in Days):', '999')
  ];

  const sale: PreferenceRow[] = [
    section('Focus Preferences'), item('Cash Sale Initial Focus:', 'detailwindow'), item('Credit Sale Initial Focus:', 'custcode'),
    section('FBR Fiscalization Settings'), item('Fiscalization Machine IP:', ''), item('Allow Zero Retail Price:', 'No'),
    section('Invoice Header Window Visibility'), item('Customer Balance:', 'Yes'), item('Reference No. 2:', 'No'), item('Reference No. 3:', 'No'), item('Reference No. 4:', 'No'), item('Sales Person:', 'No'), item('Account For:', 'No'), item('Ask Header:', 'No'), item('Price # in Cash Sale:', 'Yes'), item('Price # in Credit Sale:', 'No'), item('Item Disc. %:', 'No'), item('Message:', 'No'), item('Doctor:', 'No'), item('Show Agency:', 'No'), item('Show Vehicle:', 'No'), item('Show Ship To:', 'No'), item('Show Associated Purchase Inv. Code:', 'No'), item('Show Supplier Inv. Code:', 'No'), item('Show GRN:', 'No'), item('Show Guarantee Person:', 'No'), item('Show Sale Type:', 'No'), item('Show Item Image/Photo:', 'No'), item('Loyalty Points:', 'No'), item('Currency:', 'No'), item('Motor Vehicle:', 'No'),
    section('Item Detail Window Visibility'), item('Alternate Alias Name:', 'No'), item('Disc. % On Cash Sale:', 'Yes'), item('Pre-Discount %age:', 'No'), item('Item Flat Discount:', 'No'), item('Bonus Qty:', 'No'), item('Item Unit Sales Tax:', 'Yes'), item('Disc. % On Credit Sale:', 'Yes'), item('Claimable Disc.%:', 'No'), item('Claimable Item:', 'No'), item('Item GST %:', 'No'), item('Location:', 'No'), item('Item Packing:', 'No'), item('Packing Description:', 'No'), item('Extra Tax %:', 'No'), item('Item Description:', 'No'), item('Batch No:', 'No'), item('Purchase Price:', 'No'), item('Item Weight Per Unit:', 'No'), item('Packing Factor Per Unit:', 'No'), item('Print Warranted Invoice:', 'No')
  ];

  const saleReturn: PreferenceRow[] = [
    section('Invoice Header Window Visibility'), item('Show Account for Sale Return:', 'No'), item('Header On Sale Return:', 'No'), item('Page Size:', 'Yes'),
    section('Item Detail Window Visibility'), item('Alternate Alias Name:', 'No'), item('Packing Description:', 'No'), item('Batch:', 'Yes'), item('Expiry:', 'Yes'), item('Item Description:', 'No'), item('Total Pieces:', 'No'),
    section('Initial Column Value'), item('Sale Return Page Size:', 'Thermal Page'), item('Thermal Print Format:', 'Thermal Standard (1)'), item('Sale Return Account For:', 'Yes'),
    section('Ask Amount Paid on S/Return Saving'), item('Ask Amount Paid on S/Return Saving:', 'No'), item('Payment Mode Amt. Paid in S/Return:', 'C'), item('Payment A/C for Amt. Paid in S/Return:', ''),
    section('Other Functionality'), item('Fetch Latest Item Sales Info.:', 'No'), item('Update Avg. Price:', 'No'), item('Auto Locate Referenced Sale Return:', 'No'), item('Auto Post Sale Return Allocation:', 'Yes'), item('Ask User/Password in Sale Return:', 'No'), item('Round Item Total (decimal places):', '###,###,##0.00'), item('Show Sale Return in Activity Monitor:', 'No'), item('Copy Remarks in Item Description:', 'No'), item('Allow Empty Sale Inv # in Unposted S/R Module:', 'Yes'), item('Show Master Cashier Window in Cash S/R:', 'No'), item('Show Master Cashier Window in Credit S/R:', 'No'), item('Show Master Cashier Window in In-Patient S/R:', 'No'), item('Show Master Cashier Window in Buffer S/R:', 'No'), item('Show S/R in Cash Activity Window:', 'No'), item('Allow Same Batch Return Multiple Times:', 'Yes'), item('Fetch FBR POS Fee For Reference S/R:', 'Yes')
  ];

  const purchase: PreferenceRow[] = [
    section('Invoice Header Window Visibility'), item('Ask Header:', 'No'), item('Ask Purchase Type:', 'No'), item('Ask Purchase Order:', 'Yes'), item('Account For:', 'No'), item('Supplier Balance:', 'No'), item('Sale Return Basis:', 'No'), item('Show Agency:', 'No'), item('Show Vehicle:', 'No'), item('Show Ship To:', 'No'), item('Show Associated Sale Inv. Code:', 'No'), item('Show Dept:', 'No'), item('Show Supplier Date:', 'No'), item('Show Supplier Amount:', 'No'), item('Show GRN No.:', 'Yes'), item('Ask New Purchase Order No.:', 'No'), item('Show Supplier Inv.#:', 'Yes'), item('Ask L. C. No.:', 'No'), item('Show Item Image/Photo:', 'No'), item('Ask Credit Days:', 'No'),
    section('Item Detail Window Visibility'), item('Alternate Alias Name:', 'No'), item('Pack Units:', 'No'), item('Batch Number:', 'Yes'), item('Mfg. Date:', 'No'), item('Expiry Date:', 'Yes'), item('Pre-Disc. Price:', 'No'), item('Flat Discount:', 'No'), item('Purchase Price:', 'Yes'), item('Disc. %:', 'Yes'), item('Packing Description:', 'No'), item('P/O Qty:', 'No'), item('Bonus Quantity:', 'Yes'), item('Item Description:', 'No'), item('Sales Tax:', 'Yes'), item('Pur. Tax:', 'Yes'), item('Item Level GST %:', 'Yes'), item('Item Location:', 'No'), item('Show/Update Retail Price:', 'Yes'), item('Show Net Rate:', 'No'), item('Total Pieces:', 'No'), item('Mark-up:', 'Yes'), item('Margin%:', 'Yes'), item('Net Margin %:', 'No'), item('Manufacturer:', 'No'), item('Show Unit Weight:', 'No'), item('Show Total Weight:', 'No'), item('Extra Tax %:', 'No'), item('Batch Sale Price:', 'No'), item('Sale Disc. %:', 'No'), item('Sales Tax Schedule:', 'Yes'), item('PCT Code:', 'Yes')
  ];

  const purchaseReturn: PreferenceRow[] = [
    section('Invoice Header Window Visibility'), item('Header:', 'No'), item('Account For:', 'No'), item('Ask Pur. Invoice No.:', 'No'),
    section('Item Detail Window Visibility'), item('Alternate Alias Name:', 'No'), item('Packing Description:', 'No'), item('Item Description:', 'No'), item('Total Pieces:', 'No'), item('Show Expiry:', 'Yes'), item('Show Pack Qty.:', 'No'), item('Show Purchase Batch:', 'No'),
    section('Initial Column Value'), item('Account For:', 'Yes'), item('Auto Post:', 'No'),
    section('Ask Amount Received on P/Return Saving'), item('Ask Amount Received on P/Return Saving:', 'No'), item('Payment Mode Amt. Received in P/Return:', 'C'), item('Payment A/C for Amt. Received in P/Return:', ''),
    section('Other Functionality'), item('Price in Purchase Return:', 'No'), item('Round Item Total (decimal places):', '###,###,##0.00'), item('Ask No. of Copies to Print:', 'No'), item('Show P/Return in Activity Monitor:', 'No'), item('Allow Empty Pur. Invoice No.:', 'Yes'), item('Copy Remarks in Item Description:', 'No'), item('Update Avg. Price On P/R:', 'No'), item('Allow P/R Below Avg. Price:', 'Yes')
  ];

  const report: PreferenceRow[] = [
    section('General Report Preferences'), item('Refresh Time(Minutes):', '1'), item('Default Header On Report:', 'Ahmed Glass Industries (PVT) LTD'), item('Apply Default Date for Report Arg. Window?', 'No'), item('Default Start Date:', '2005-07-12 15:37:01'), item('Allow Print Setup:', 'No'), item('Show Account in Reports:', 'All Accounts'),
    section('Terms'), item('Report Term 1:', ''), item('Report Term 2:', ''), item('Report Term 3:', ''), item('Report Term 4:', ''), item('Report Term 5:', ''), item('Report Term 6:', '')
  ];

  const basicData: PreferenceRow[] = [
    section('Basic Data Preferences Settings'), item('Manufacturer:', 'A.J. & COMPANY'), item('Item Category:', 'DEFAULT CATEGORY'), item('Item Class:', 'DEFAULT CLASS'), item('Item Packing:', 'DEFAULT PACKING'), item('Associated Godown:', 'DEFAULT GODOWN'), item('Lock Item Disc. (%):', 'No'), item('Lock Item Sale Price:', 'No'), item('Allow Due Default Value in Item Basic Data:', 'No'), item('Ask User/Password In Item:', 'No'), item('Ask User/Password In Patient Registration:', 'No'), item('Sales Person Scope For Sub Area:', 'Allow Manf. Wise Multiple S/Person in a Sub Area')
  ];

  const quotation: PreferenceRow[] = [
    section('Header/Footer Column Visibility'), item('Ask Header:', 'No'), item('Delivery Days:', 'No'), item('Validity Days:', 'No'), item('Payment To:', 'No'),
    section('Detail Window Column Visibility'), item('Remote Net Sale Qty:', 'No'), item('Remote Stock in Hand:', 'No'), item('Remote Re-Order Qty:', 'No'), item('Remote Optimum Qty:', 'No'), item('Remote Min Qty:', 'No'), item('Remote P/O Qty:', 'No'), item('Manufacturer Name:', 'No'), item('Pack Units:', 'No'), item('Show P/O Rate:', 'No'), item('Show P/O Discount (%):', 'No'), item('Show Special Rate:', 'No'), item('Show Ref. No.:', 'No'), item('Claimable Discount %:', 'No'), item('Color:', 'No'),
    section('Initial Column Settings'), item('Quantity Denomination:', '1'),
    section('Other Functionality'), item('Show Quotation in Activity Monitor:', 'No'), item('Allow Duplicate Items:', 'No'), item('Allow Quotation On Zero Price:', 'No'), item('Fetch Latest Sale Price:', 'No'), item('Allow Quotation Below Avg. Price:', 'Yes'), item('Allow Quotation Below RPP:', 'Yes'), item('Allow Quotation Above Avg. Price:', 'Yes'), item('Drop Box - Auto Fetch Prices Of Branch P/O:', 'Yes'), item('Drop Box - Auto Fetch Discounts Of Branch P/O:', 'Yes'),
    section('Terms'), item('Line1:', ''), item('Line2:', ''), item('Line3:', ''), item('Line4:', ''), item('Line5:', ''), item('Line6:', ''), item('Line7:', ''), item('Line8:', '')
  ];

  const schedule: PreferenceRow[] = [
    section('Auto Database Backup Schedule'), item('Activate:', 'No'), item('Once At:', ''), item('Hour:', ''), item('Minute:', ''), item('Second:', ''), item('Every:', ''),
    section('Auto Cash Sale Posting Schedule'), item('Activate:', 'No'), item('Post Cash Sale Invoices:', ''), item('Minutes old:', ''), item('Occurs Every:', ''),
    section('Auto Credit Sale Posting Schedule'), item('Activate:', 'No'), item('Post Credit Sale Invoices:', ''), item('Minutes old:', ''), item('Occurs Every:', ''),
    section('Auto Satisfy Due Service Schedule'), item('Activate:', 'No'), item('Once At:', ''), item('Hour:', ''), item('Minute:', ''), item('Second:', ''), item('Every:', ''),
    section('In-Patient Recurring Services Schedule'), item('Activate:', 'No'), item('Once At:', ''), item('Hour:', ''), item('Minute:', ''), item('Second:', ''), item('Every:', ''),
    section('Checked-In Guest Services Schedule'), item('Activate:', 'No'), item('Once a day at:', ''), item('Hour:', ''), item('Minute:', ''), item('Second:', '')
  ];

  const adjustment: PreferenceRow[] = [
    section('Column Visibility'), item('Show Header:', 'No'), item('Show Update Avg. Price Column:', 'No'), item('Adjustment Qty:', 'No'), item('Alternate Alias Name:', 'No'), item('Show Batch:', 'Yes'), item('Show Expiry:', 'Yes'),
    section('Initial Column Value'), item('Update Avg. Price:', 'Yes'),
    section('Other Functionality'), item('Ask User/Password in Adjustment:', 'No'), item('In Adj. Buffer, Pending Due Effect:', 'No'), item('Show Adjustments in Activity Monitor:', 'No')
  ];

  const purchaseOrder: PreferenceRow[] = [
    section('Invoice Header Window Visibility'), item('Show Supplier Reference:', 'No'), item('Show Delivery Place:', 'No'), item('Show Usage Palace:', 'No'), item('Show Required Date:', 'No'), item('Show Remarks 2:', 'No'), item('Show Purchase Type:', 'Yes'), item('Show Remarks 3:', 'No'),
    section('Item Detail Window Visibility'), item('Show Item Remarks/Description:', 'No'), item('Show Item Sale Tax:', 'No'), item('Show Item GST %:', 'No'), item('Show Item Flat Discount:', 'No'), item('Show Disc. Perc. 2.:', 'No'), item('Item Alert:', 'No'),
    section('Invoice Footer Window Visibility'), item('Show Invoice Discount (%):', 'Yes'), item('Show Misc.Charges:', 'No'), item('Show Invoice GST (%):', 'No'), item('Show Invoice Flat Discount:', 'No'), item('Show Grand Total:', 'Yes'),
    section('Other Functionality'), item('Required Packs Fraction Treatment:', 'Rounding'), item('Auto Update Transit Stock at Posting:', 'No'), item('Update Re-Order Qty at Saving:', 'No'), item('Page Size for print out:', 'Thermal Page'), item('Apply Supplier Items Only Option', 'Yes'), item('Default Purchase Order Category', '1'), item('Update Minimum Qty at Saving:', 'No'), item('Apply Associated Quotation On Save', 'No'), item('Update Optimum Qty at Saving:', 'No'), item('Show Pur. Order in Activity Monitor:', 'No'), item('Show Un-Posted P/Ret. on P/O Saving:', 'No'), item('Copy Remarks in Item Description', 'No'), item('Enforce Supplier/Item Association On P/O saving:', 'No'), item('Fetch Last Purchase Info. in P/O:', 'No'), item('P/O Supplier Consideration:', 'Select Only Min. Priority Supplier'), item('Apply Supplier Associated Quotation:', 'No'), item('Consider Issue Qty:', 'No'), item('Consider Receipt Qty:', 'No'),
    section('Terms'), item('Line1:', ''), item('Line2:', ''), item('Line3:', ''), item('Line4:', ''), item('Line5:', ''), item('Line6:', ''), item('Line7:', ''), item('Line8:', ''),
    section('Purchase Order Footer (Print out)'), item('Purchase Order Footer:', '')
  ];

  const others: PreferenceRow[] = [
    section('Activity Monitor'), item('Preferred Printer for Activity Monitor:', '<Default Printer>'), item('SMS Expiry (in Hours):', '24'), item('Refresh Time (Seconds):', '15'), item('Activity Period (Minutes):', '30'),
    section('Others'), item('Mfg. Distributor Code:', ''), item('Distributor Code:', '90'), item('Retail Price(%) for Sale Inv.:', '0.00'), item('Output File(s) Location :', 'C:\\'), item('Default Location for [Save As PDF]:', 'C:\\'), item('Synchronize Parent Server Stock:', 'No'), item('Keep Child Server Invoice No Intact:', 'No'), item('Copy Child Server Invoice No in Ref # 1:', 'No'), item('Prepare Purchase Dump Before Data Migration:', 'No'), item('Prepare Sales Dump Before Data Migration:', 'No'), item('Save Sale History Before Data Migration:', 'Yes'), item('Auto Purge Posted Sale Time(Minutes)', '60'), item('Select Image Scan Tool:', 'Dynamic Twain')
  ];

  const pointOfSale: PreferenceRow[] = [
    section('POS Header Window Column Visibility'), item('Invoice Size:', 'No'), item('Sales Person:', 'No'), item('Loyalty Points:', 'No'), item('Delivered By:', 'No'),
    section('POS Detail Window Column Visibility'), item('Item Discount %:', 'Yes'), item('Item Flat Discount:', 'Yes'), item('Item GST %:', 'Yes'), item('Item Unit Sales Tax:', 'No'),
    section('POS Footer Window Column Visibility'), item('Invoice GST %:', 'No'), item('Misc. Charges:', 'Yes'), item('Invoice Discount %:', 'Yes'), item('Invoice Flat Discount:', 'Yes'),
    section('POS Other Functionality'), item('Prompt For Zero Stock:', 'No'), item('Sale Sets/Deals In POS:', 'No'), item('Auto Production In POS:', 'No'), item('Ask Associate Sale Invoices on Saving:', 'No'), item('Check Due Date on Saving:', 'No'), item('Show Cashier Window:', 'Yes'), item('Show Master Cashier Window In POS', 'No'), item('Print Store Summary With POS Sale Inv.', 'No'), item('Ask User/Password In POS', 'Yes'), item('Reset Inv. Balance Field in Footer On Saving', 'Yes'), item('POS List View Retrieval Limit (days):', '1'), item('POS List View Max. Retrieval Limit (days):', '5'), item('Show Sale in Cashier Activity Window:', 'No'), item('Lock Sale Qty/Bonus:', 'No'), item('Lock Item Name:', 'No'), item('Allow CTRL+D :', 'Yes'), item('Must Save Invoice on Exit:', 'No'),
    section('POS Settings - LCD Display'), item('Default LCD Config:', '2'), item('Use LCD Display:', 'No'), item('LCD Display Manufacturer/Model:', ''),
    section('POS Settings - Cash Drawer'), item('Default Cash Drawer COM Port', '1'), item('Use Cash Drawer:', 'No'), item('Use Cash Drawer With Printer:', 'No'), item('Cash Drawer Printer Name:', ''),
    section('POS Settings - BarCode Printer'), item('Use BarCode Printer:', 'No'), item('BarCode Printer Name/Model:', '')
  ];

  const cashierJobActivity: PreferenceRow[] = [
    section('Column Visibility'), item('Date:', 'Yes'), item('Header Inv. No.:', 'Yes'), item('Remarks:', 'Yes'), item('User:', 'Yes'), item('Machine Name:', 'Yes'), item('Posted:', 'Yes'), item('GL Voucher Code:', 'Yes'), item('Type/Module:', 'Yes'), item('Invoice Number:', 'Yes'), item('Invoice Amount:', 'Yes'), item('Cash Tendered:', 'Yes'), item('Cash Charged:', 'Yes'), item('Balance/Cash Back:', 'Yes'), item('Cash Account:', 'Yes'), item('Category:', 'Yes'), item('Account Title:', 'Yes'), item('Supervised:', 'Yes'), item('Allow Selection:', 'Yes'),
    section('Other Functionality'), item('Refresh Time (in seconds):', '15'), item('Mode:', 'Active Entry'), item('Auto Fill Cash Charged:', 'No'), item('Show Empty Doc. Code by Default:', 'No'), item('Keep Cash Window Always Open:', 'No'), item('Show Saving/Supervised Messages:', 'No')
  ];

  const email: PreferenceRow[] = [item('SMTP Server:', 'smtp.gmail.com'), item('SMTP Port:', '465'), item('From/Sender Name:', 'MyName'), item('Email User ID:', 'myaddress@gmail.com'), item('Email Password:', '********'), item('SMTP Server Requires Authentication (User/Password):', 'No'), item('SMTP Encryption Type:', 'None'), item('Email Subject:', ''), item('Email Body:', '')];
  const sms: PreferenceRow[] = [section('SMS Preferences'), item('SMS Method:', 'Mobile Device'), item('Web SMS Provider:', 'Zong'), item('Web SMS User ID:', ''), item('Web SMS Password:', ''), item('Web SMS Mask:', ''), item('Web SMS API Key:', '')];
  const dashboard: PreferenceRow[] = [section('Dashboard Preferences'), item('Summary Analysis Days Lim.:', '30'), item('Sales Analysis Days Lim.:', '30'), item('Inventory Breakup On:', 'Category'), item('Purchase Analysis Days Lim.:', '30'), item('Receipt Analysis Days Lim.:', '30'), item('Issue Analysis Days Lim.:', '30'), item('Adjustment Analysis Days Lim.:', '30'), item('Godown Transfer Analysis Days Lim.:', '30'), item('Accounts Analysis Days Lim.:', '30'), item('Service Analysis Days Lim.:', '30')];

  const preferencesByTab: Record<string, PreferenceRow[]> = { General: general, Sale: sale, 'Sale Return': saleReturn, Purchase: purchase, 'Purchase Return': purchaseReturn, Report: report, BasicData: basicData, Quotation: quotation, Schedule: schedule, Adjustment: adjustment, 'Purchase Order': purchaseOrder, Others: others, 'Point of Sale': pointOfSale, 'Cashier Job Activity': cashierJobActivity, Email: email, SMS: sms, Dashboard: dashboard };

  let activeTab = 'General';
  let message = '';
  let error = '';
  let busy = false;
  let interactive = false;
  let values = general.map((row) => row.value);
  let originalValues = [...values];
  let activeRows: PreferenceRow[] = general;
  const api = new AbuzarApi();
  const baselineClasses: Record<string, string> = {
    General: 'legacy-preferences-general-baseline', Sale: 'legacy-preferences-sale-baseline', 'Sale Return': 'legacy-preferences-sale-return-baseline', Purchase: 'legacy-preferences-purchase-baseline', 'Purchase Return': 'legacy-preferences-purchase-return-baseline', Report: 'legacy-preferences-report-baseline', BasicData: 'legacy-preferences-basicdata-baseline', Quotation: 'legacy-preferences-quotation-baseline', Schedule: 'legacy-preferences-schedule-baseline', Adjustment: 'legacy-preferences-adjustment-baseline', 'Purchase Order': 'legacy-preferences-purchase-order-baseline', Others: 'legacy-preferences-others-baseline', 'Point of Sale': 'legacy-preferences-point-of-sale-baseline', 'Cashier Job Activity': 'legacy-preferences-cashier-job-activity-baseline', Email: 'legacy-preferences-email-baseline', SMS: 'legacy-preferences-sms-baseline', Dashboard: 'legacy-preferences-dashboard-baseline'
  };
  $: activeRows = preferencesByTab[activeTab] ?? [];
  $: baselineClass = interactive ? '' : (baselineClasses[activeTab] ?? '');

  function enableInteractive() {
    interactive = true;
  }

  function inputId(tab: string, index: number): string {
    return `preference-value-${tab.replace(/[^a-z0-9]+/gi, '-').toLowerCase()}-${index}`;
  }

  function selectTab(tab: string) {
    activeTab = tab;
    activeRows = preferencesByTab[tab] ?? [];
    values = activeRows.map((row) => row.value);
    originalValues = [...values];
    interactive = false;
    message = `${tab} preferences selected.`;
    void loadCategory(tab);
  }

  onMount(() => { void loadCategory(activeTab); });

  async function loadCategory(category: string) {
    error = '';
    try {
      const result = await api.preferences(category);
      if (category !== activeTab) return;
      const rows = preferencesByTab[category] ?? [];
      values = rows.map((row) => result.items.find((preference) => preference.caption === row.caption)?.value ?? row.value);
      originalValues = [...values];
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = 'Preferences could not be loaded; local defaults remain available.';
    }
  }

  async function save() {
    busy = true;
    message = '';
    error = '';
    try {
      await api.savePreferences(activeTab, activeRows.flatMap((row, index) => row.section ? [] : [{ caption: row.caption, value: values[index] ?? '', position: index }]));
      originalValues = [...values];
      message = `${activeTab} preferences saved for the current tenant.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Preferences could not be saved.';
    } finally {
      busy = false;
    }
  }

  function cancel() {
    values = [...originalValues];
    message = 'Changes cancelled.';
  }

  function edit(row: PreferenceRow, index: number) {
    enableInteractive();
    message = `${row.caption} editor opened.`;
    requestAnimationFrame(() => document.getElementById(inputId(activeTab, index))?.focus());
  }

  function displayNumber(index: number): number {
    return activeRows.slice(0, index + 1).filter((row) => !row.section).length;
  }

  function setValue(index: number, value: string) {
    values = values.map((current, currentIndex) => currentIndex === index ? value : current);
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA - ABUZAR V3 - Preferences</title></svelte:head>
<main class={`legacy-preferences-page ${baselineClass}`} onpointerdown={enableInteractive} onfocusin={enableInteractive}><section class="legacy-preferences-window" aria-label="Preferences">
  <header class="legacy-preferences-titlebar"><a href="/app/legacy" aria-label="Back to main window">&larr;</a><h1>Preferences</h1></header>
  <nav class="legacy-preferences-tabs" aria-label="Preference categories">{#each tabs as tab}<button type="button" class:active={activeTab === tab} onclick={() => selectTab(tab)}>{tab}</button>{/each}</nav>
  <div class="legacy-preferences-body">
    {#if activeTab === 'Schedule'}
      <div class="legacy-preferences-schedule-form" aria-label="Schedule preferences">
        <fieldset><legend>Auto Database Backup Schedule</legend><label><input type="checkbox" checked={values[1] === 'Yes'} onchange={(event) => setValue(1, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Once At:<input value={values[2]} oninput={(event) => setValue(2, event.currentTarget.value)} /></label><label>Hr:<input value={values[3]} oninput={(event) => setValue(3, event.currentTarget.value)} /></label><label>Min:<input value={values[4]} oninput={(event) => setValue(4, event.currentTarget.value)} /></label><label>Sec:<input value={values[5]} oninput={(event) => setValue(5, event.currentTarget.value)} /></label><label>Every:<input value={values[6]} oninput={(event) => setValue(6, event.currentTarget.value)} /></label></fieldset>
        <fieldset><legend>Auto Cash Sale Posting Schedule</legend><label><input type="checkbox" checked={values[8] === 'Yes'} onchange={(event) => setValue(8, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Post Cash Sale Invoices:<input value={values[9]} oninput={(event) => setValue(9, event.currentTarget.value)} /></label><label>Minutes old:<input value={values[10]} oninput={(event) => setValue(10, event.currentTarget.value)} /></label><label>Occurs Every:<input value={values[11]} oninput={(event) => setValue(11, event.currentTarget.value)} /></label></fieldset>
        <fieldset><legend>Auto Credit Sale Posting Schedule</legend><label><input type="checkbox" checked={values[13] === 'Yes'} onchange={(event) => setValue(13, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Post Credit Sale Invoices:<input value={values[14]} oninput={(event) => setValue(14, event.currentTarget.value)} /></label><label>Minutes old:<input value={values[15]} oninput={(event) => setValue(15, event.currentTarget.value)} /></label><label>Occurs Every:<input value={values[16]} oninput={(event) => setValue(16, event.currentTarget.value)} /></label></fieldset>
        <fieldset><legend>Auto Satisfy Due Service Schedule</legend><label><input type="checkbox" checked={values[18] === 'Yes'} onchange={(event) => setValue(18, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Once At:<input value={values[19]} oninput={(event) => setValue(19, event.currentTarget.value)} /></label><label>Hr:<input value={values[20]} oninput={(event) => setValue(20, event.currentTarget.value)} /></label><label>Min:<input value={values[21]} oninput={(event) => setValue(21, event.currentTarget.value)} /></label><label>Sec:<input value={values[22]} oninput={(event) => setValue(22, event.currentTarget.value)} /></label><label>Every:<input value={values[23]} oninput={(event) => setValue(23, event.currentTarget.value)} /></label></fieldset>
        <fieldset><legend>In-Patient Recurring Services Schedule</legend><label><input type="checkbox" checked={values[25] === 'Yes'} onchange={(event) => setValue(25, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Once At:<input value={values[26]} oninput={(event) => setValue(26, event.currentTarget.value)} /></label><label>Hr:<input value={values[27]} oninput={(event) => setValue(27, event.currentTarget.value)} /></label><label>Min:<input value={values[28]} oninput={(event) => setValue(28, event.currentTarget.value)} /></label><label>Sec:<input value={values[29]} oninput={(event) => setValue(29, event.currentTarget.value)} /></label><label>Every:<input value={values[30]} oninput={(event) => setValue(30, event.currentTarget.value)} /></label></fieldset>
        <fieldset><legend>Checked-In Guest Services Schedule</legend><label><input type="checkbox" checked={values[32] === 'Yes'} onchange={(event) => setValue(32, event.currentTarget.checked ? 'Yes' : 'No')} />Activate</label><label>Once a day at:<input value={values[33]} oninput={(event) => setValue(33, event.currentTarget.value)} /></label><label>Hr:<input value={values[34]} oninput={(event) => setValue(34, event.currentTarget.value)} /></label><label>Min:<input value={values[35]} oninput={(event) => setValue(35, event.currentTarget.value)} /></label><label>Sec:<input value={values[36]} oninput={(event) => setValue(36, event.currentTarget.value)} /></label></fieldset>
      </div>
    {:else if activeTab === 'Email'}
      <div class="legacy-preferences-email-form" aria-label="Email preferences">
        <label>SMTP Server:<input value={values[0]} oninput={(event) => setValue(0, event.currentTarget.value)} /></label><label>SMTP Port:<input value={values[1]} oninput={(event) => setValue(1, event.currentTarget.value)} /></label><label>From/Sender Name:<input value={values[2]} oninput={(event) => setValue(2, event.currentTarget.value)} /></label><label>Email User ID:<input value={values[3]} oninput={(event) => setValue(3, event.currentTarget.value)} /></label><label>Email Password:<input type="password" value={values[4]} oninput={(event) => setValue(4, event.currentTarget.value)} /></label><label class="legacy-preferences-checkbox"><input type="checkbox" checked={values[5] === 'Yes'} onchange={(event) => setValue(5, event.currentTarget.checked ? 'Yes' : 'No')} />SMTP Server Requires Authentication (User/Password)</label><label>SMTP Encryption Type:<select value={values[6]} onchange={(event) => setValue(6, event.currentTarget.value)}><option>None</option><option>SSL</option><option>TLS</option></select></label><label>Email Subject:<input value={values[7]} oninput={(event) => setValue(7, event.currentTarget.value)} /></label><label>Email Body:<input value={values[8]} oninput={(event) => setValue(8, event.currentTarget.value)} /></label>
      </div>
    {:else}
      <table><thead><tr><th>Sr. #</th><th>Preference Caption</th><th>Preference Value</th><th></th></tr></thead><tbody>
        {#each activeRows as row, index}
          {#if row.section}<tr class="section"><td colspan="4">{row.caption}</td></tr>{:else}<tr><td>{displayNumber(index)}</td><td>{row.caption}</td><td><input id={inputId(activeTab, index)} bind:value={values[index]} aria-label={row.caption} /></td><td><button type="button" aria-label={`Edit ${row.caption}`} onclick={() => edit(row, index)}>...</button></td></tr>{/if}
        {/each}
      </tbody></table>
    {/if}
  </div>
  <footer class="legacy-preferences-footer"><button type="button" onclick={save} disabled={busy}>Save</button><button type="button" onclick={cancel}>Cancel</button>{#if error}<span class="legacy-preferences-error" role="alert">{error}</span>{:else}<span role="status">{message}</span>{/if}</footer>
</section></main>
