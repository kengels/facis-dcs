import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'

const source = (relativePath: string) => readFileSync(new URL(relativePath, import.meta.url), 'utf8')

void describe('contract creation UI', () => {
  const newContractView = source('../../src/views/contract/NewContractView.vue')
  const viewContractView = source('../../src/views/contract/ViewContractView.vue')
  const dcsJsonLdModel = source('../../src/models/dcs-jsonld.ts')
  const contractDetailsEditor = source(
    '../../src/modules/contract-workflow-engine/components/ContractDetailsEditor.vue',
  )

  void it('uses the existing details editor instead of ad-hoc prefill fields', () => {
    assert.doesNotMatch(newContractView, /initial(?:Name|Party|Asset|Policy|Evidence)/)
    assert.doesNotMatch(newContractView, /contract-create-(?:party|asset|policy|evidence)/)
    assert.doesNotMatch(newContractView, /data-test-id="contract-create-name"/)
    assert.match(contractDetailsEditor, /data-test-id="contract-create-name"/)
    assert.match(contractDetailsEditor, /:data-test-key="contract\.did"/)
  })

  void it('keeps the optional parent selector addressable', () => {
    assert.match(newContractView, /data-test-id="contract-create-parent"/)
  })

  void it('does not retain output fields or model properties for the removed prefill data', () => {
    assert.doesNotMatch(viewContractView, /contract-(?:party|asset|policy|evidence)-list/)
    assert.doesNotMatch(dcsJsonLdModel, /'dcs:(?:parties|assets|policyTypes|evidence)'/)
  })
})
