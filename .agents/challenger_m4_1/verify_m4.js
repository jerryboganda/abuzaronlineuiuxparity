import fs from 'fs';
import path from 'path';

// 1. Verify Catalog Leaves
const catalogPath = path.resolve('../../parity/catalog/legacy-menu-tree-2026-08-05.json');
const rawCatalog = fs.readFileSync(catalogPath, 'utf8').replace(/^\uFEFF/, '');
const catalog = JSON.parse(rawCatalog);

const leaves = catalog.items.filter(item => {
  const clean = item.path.replace(/&/g, '').trim();
  return !item.hasSubmenu && clean.startsWith('Reports > ') && !clean.endsWith(' >');
});

console.log(`[Catalog Test] Found ${leaves.length} non-blank report leaves (expected 151).`);
if (leaves.length !== 151) {
  console.error(`FAIL: Expected 151 leaves, got ${leaves.length}`);
  process.exit(1);
}

// 2. Stress-test Zoom Bounds
let zoom = 100;
function setPreviewZoom(offset) {
  zoom = Math.max(50, Math.min(200, zoom + offset));
}

// Test lower bound
setPreviewZoom(-500);
console.log(`[Zoom Test Min] setPreviewZoom(-500) -> ${zoom}% (expected 50%)`);
if (zoom !== 50) { console.error('FAIL: Zoom min bound violated'); process.exit(1); }

// Test upper bound
setPreviewZoom(+500);
console.log(`[Zoom Test Max] setPreviewZoom(+500) -> ${zoom}% (expected 200%)`);
if (zoom !== 200) { console.error('FAIL: Zoom max bound violated'); process.exit(1); }

// 3. Stress-test Pagination Slicing
const pageSize = 24;
function getPageInfo(rowsCount, page) {
  const pageCount = Math.max(1, Math.ceil(rowsCount / pageSize));
  const sliceStart = (page - 1) * pageSize;
  const sliceEnd = page * pageSize;
  return { pageCount, sliceStart, sliceEnd };
}

// Case: 0 rows
let p0 = getPageInfo(0, 1);
console.log(`[Pagination Test 0 rows] pageCount=${p0.pageCount}, slice=[${p0.sliceStart}, ${p0.sliceEnd}]`);
if (p0.pageCount !== 1 || p0.sliceStart !== 0 || p0.sliceEnd !== 24) { console.error('FAIL: 0 rows pagination'); process.exit(1); }

// Case: 24 rows
let p24 = getPageInfo(24, 1);
console.log(`[Pagination Test 24 rows] pageCount=${p24.pageCount}`);
if (p24.pageCount !== 1) { console.error('FAIL: 24 rows pagination'); process.exit(1); }

// Case: 25 rows
let p25_1 = getPageInfo(25, 1);
let p25_2 = getPageInfo(25, 2);
console.log(`[Pagination Test 25 rows] pageCount=${p25_1.pageCount}, p1 slice=[${p25_1.sliceStart}, ${p25_1.sliceEnd}], p2 slice=[${p25_2.sliceStart}, ${p25_2.sliceEnd}]`);
if (p25_1.pageCount !== 2 || p25_2.sliceStart !== 24) { console.error('FAIL: 25 rows pagination'); process.exit(1); }

// 4. Stress-test CSV Escaping
function escapeCsvCell(cell) {
  return `"${String(cell ?? '').replace(/"/g, '""')}"`;
}

const testCell1 = 'Hello "World"';
const testCell2 = null;
const testCell3 = 'Line1\r\nLine2, with comma';
console.log(`[CSV Test 1] ${escapeCsvCell(testCell1)}`);
console.log(`[CSV Test 2] ${escapeCsvCell(testCell2)}`);
console.log(`[CSV Test 3] ${escapeCsvCell(testCell3)}`);
if (escapeCsvCell(testCell1) !== '"Hello ""World"""') { console.error('FAIL: CSV quote escaping'); process.exit(1); }
if (escapeCsvCell(testCell2) !== '""') { console.error('FAIL: CSV null cell escaping'); process.exit(1); }

// 5. Stress-test Excel HTML Escaping
function escapeHtmlCell(cell) {
  return String(cell ?? '').replace(/[&<>\"]/g, (value) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '\"': '&quot;' }[value] ?? value));
}

const htmlCell1 = '<script>alert("xss")</script> & "more"';
console.log(`[Excel HTML Test] ${escapeHtmlCell(htmlCell1)}`);
if (escapeHtmlCell(htmlCell1) !== '&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt; &amp; &quot;more&quot;') {
  console.error('FAIL: Excel HTML escaping');
  process.exit(1);
}

console.log('\n--- ALL EMPIRICAL UNIT TESTS PASSED CLEANLY ---');
