import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { TemplateType } from '../../src/modules/template-repository/models/contract-template.ts'
import { isReusableComponentTemplate } from '../../src/modules/template-repository/utils/is-reusable-component-template.ts'
import { TemplateState } from '../../src/types/contract-template-state.ts'

void describe('isReusableComponentTemplate', () => {
  void it('accepts registered and published component templates', () => {
    assert.equal(
      isReusableComponentTemplate({ template_type: TemplateType.component, state: TemplateState.registered }),
      true,
    )
    assert.equal(
      isReusableComponentTemplate({ template_type: TemplateType.component, state: TemplateState.published }),
      true,
    )
  })

  void it('does not accept approved component templates', () => {
    assert.equal(
      isReusableComponentTemplate({ template_type: TemplateType.component, state: TemplateState.approved }),
      false,
    )
  })

  void it('does not accept contract templates in a reusable component state', () => {
    assert.equal(
      isReusableComponentTemplate({ template_type: TemplateType.contractTemplate, state: TemplateState.registered }),
      false,
    )
  })
})
