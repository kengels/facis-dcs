import { isReusableComponentTemplate } from './is-reusable-component-template.ts'
import { isDcsTemplateData } from '../../../models/dcs-jsonld.ts'
import type { ContractTemplate, PartialContractTemplate, SubTemplateSnapshot } from '@/models/contract-template'

const selectionError = (message: string): Error => new Error(`Dependency selection failed: ${message}`)

export const selectComponentSnapshot = async (
  rawReference: string,
  availableTemplates: readonly PartialContractTemplate[],
  retrieveTemplate: (did: string) => Promise<ContractTemplate | null>,
): Promise<SubTemplateSnapshot> => {
  const reference = rawReference.trim()
  if (!reference) {
    throw selectionError('enter a component template DID.')
  }

  const available = availableTemplates.find((template) => template.did === reference)
  if (!available) {
    throw selectionError('select an exact DID from the available component templates.')
  }

  const template = await retrieveTemplate(reference)
  if (template?.did !== reference) {
    throw selectionError('the selected component could not be loaded.')
  }
  if (!isReusableComponentTemplate(template)) {
    throw selectionError('the selected component is no longer registered or published.')
  }
  if (!isDcsTemplateData(template.template_data)) {
    throw selectionError('the selected component has no complete template data.')
  }

  return {
    did: template.did,
    version: template.version,
    document_number: template.document_number,
    name: template.name,
    description: template.description,
    template_data: template.template_data,
  }
}
