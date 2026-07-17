import { TemplateState } from '../../../types/contract-template-state.ts'
import type { PartialContractTemplate } from '@/models/contract-template'

type TemplateLifecycleCandidate = Pick<PartialContractTemplate, 'state'>

export const canManagerRegisterTemplate = (isManager: boolean, template: TemplateLifecycleCandidate): boolean =>
  isManager && template.state === TemplateState.approved

export const canManagerPublishTemplate = (isManager: boolean, template: TemplateLifecycleCandidate): boolean =>
  isManager && template.state === TemplateState.registered
