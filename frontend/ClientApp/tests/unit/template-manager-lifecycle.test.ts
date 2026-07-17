import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { TemplateType } from '../../src/modules/template-repository/models/contract-template.ts'
import {
  canManagerPublishTemplate,
  canManagerRegisterTemplate,
} from '../../src/modules/template-repository/utils/template-manager-lifecycle.ts'
import { TemplateState } from '../../src/types/contract-template-state.ts'

void describe('template manager lifecycle actions', () => {
  for (const templateType of [TemplateType.component, TemplateType.contractTemplate]) {
    void it(`allows a manager to register an approved ${templateType} template`, () => {
      const template = { state: TemplateState.approved, template_type: templateType }

      assert.equal(canManagerRegisterTemplate(true, template), true)
      assert.equal(canManagerRegisterTemplate(false, template), false)
    })

    void it(`allows a manager to publish a registered ${templateType} template`, () => {
      const template = { state: TemplateState.registered, template_type: templateType }

      assert.equal(canManagerPublishTemplate(true, template), true)
      assert.equal(canManagerPublishTemplate(false, template), false)
    })
  }

  void it('rejects lifecycle actions in the wrong state', () => {
    assert.equal(canManagerRegisterTemplate(true, { state: TemplateState.registered }), false)
    assert.equal(canManagerPublishTemplate(true, { state: TemplateState.approved }), false)
  })
})
