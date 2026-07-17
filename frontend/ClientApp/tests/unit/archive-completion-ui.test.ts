import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'

const source = (relativePath: string) => readFileSync(new URL(relativePath, import.meta.url), 'utf8')

void describe('completed archive UI', () => {
  const view = source('../../src/views/archive/ArchiveDashboardView.vue')
  const service = source('../../src/services/archive-service.ts')

  void it('exposes operational alerts and retention management', () => {
    assert.match(view, /data-test-id="archive-alerts"/)
    assert.match(view, /data-test-id="archive-alert-acknowledge"/)
    assert.match(view, /data-test-id="archive-alert-preferences-save"/)
    assert.match(view, /data-test-id="archive-retention-save"/)
    assert.match(service, /\/archive\/alerts/)
    assert.match(service, /\/archive\/monitoring-preferences/)
    assert.match(service, /\/archive\/retention/)
  })

  void it('exposes ontology metadata filters and saved searches', () => {
    for (const id of [
      'archive-party-filter',
      'archive-contract-type-filter',
      'archive-jurisdiction-filter',
      'archive-parent-filter',
      'archive-valid-from-filter',
      'archive-valid-to-filter',
    ]) {
      assert.match(view, new RegExp(`data-test-id="${id}"`))
    }
    assert.match(view, /data-test-id="archive-saved-query-save"/)
    assert.match(service, /\/archive\/saved-queries/)
  })

  void it('supports component visibility, bulk renewal and legal exports', () => {
    assert.match(view, /data-test-id="archive-components"/)
    assert.match(view, /data-test-id="archive-bulk-renew"/)
    assert.match(view, /data-test-id="archive-search-export-csv"/)
    assert.match(service, /\/archive\/components/)
  })
})
