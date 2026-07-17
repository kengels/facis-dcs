import { TemplateState } from '../../../types/contract-template-state.ts'
import { TemplateType } from '../models/contract-template.ts'
import type { PartialContractTemplate } from '@/models/contract-template'

type ComponentTemplateCandidate = Pick<PartialContractTemplate, 'template_type' | 'state'>

export const isReusableComponentTemplate = (template: ComponentTemplateCandidate): boolean =>
  template.template_type === TemplateType.component &&
  (template.state === TemplateState.registered || template.state === TemplateState.published)
