import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { TemplateType } from '../../src/modules/template-repository/models/contract-template.ts'
import { selectComponentSnapshot } from '../../src/modules/template-repository/utils/component-snapshot-selection.ts'
import { TemplateState } from '../../src/types/contract-template-state.ts'
import type { ContractTemplate } from '../../src/models/contract-template.ts'
import type { DcsTemplateData } from '../../src/models/dcs-jsonld.ts'

const did = 'did:web:components.example:reusable-terms'
const templateData = {
  '@type': 'dcs:ContractTemplate',
  'dcs:metadata': {},
  'dcs:documentStructure': {},
  'dcs:contractData': [],
  'dcs:policies': [],
} as DcsTemplateData

const buildComponentTemplate = (overrides: Partial<ContractTemplate> = {}): ContractTemplate => ({
  did,
  created_by: 'creator',
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  version: 2,
  name: 'Reusable terms',
  state: TemplateState.registered,
  template_type: TemplateType.component,
  template_data: templateData,
  ...overrides,
})

void describe('selectComponentSnapshot', () => {
  void it('uses an exact trimmed available DID and returns the complete authoritative snapshot', async () => {
    const available = buildComponentTemplate({ template_data: undefined })
    let retrievedDID = ''

    const snapshot = await selectComponentSnapshot(`  ${did}  `, [available], (reference) => {
      retrievedDID = reference
      return Promise.resolve(buildComponentTemplate())
    })

    assert.equal(retrievedDID, did)
    assert.equal(snapshot.did, did)
    assert.equal(snapshot.name, 'Reusable terms')
    assert.equal(snapshot.template_data, templateData)
  })

  void it('rejects a DID that is not an exact available match without retrieving it', async () => {
    let retrieved = false

    await assert.rejects(
      selectComponentSnapshot(did.toUpperCase(), [buildComponentTemplate()], () => {
        retrieved = true
        return Promise.resolve(buildComponentTemplate())
      }),
      /exact DID/,
    )
    assert.equal(retrieved, false)
  })

  void it('rejects an authoritative response that is no longer reusable', async () => {
    await assert.rejects(
      selectComponentSnapshot(did, [buildComponentTemplate()], () =>
        Promise.resolve(buildComponentTemplate({ state: TemplateState.approved })),
      ),
      /no longer registered or published/,
    )
  })

  void it('rejects an authoritative response without complete template data', async () => {
    await assert.rejects(
      selectComponentSnapshot(did, [buildComponentTemplate()], () =>
        Promise.resolve(buildComponentTemplate({ template_data: undefined })),
      ),
      /no complete template data/,
    )
  })
})
