import assert from 'node:assert/strict'
import { it } from 'node:test'
import { extractErrorMessage } from '../../src/utils/error-message.ts'

void it('preserves the backend message from an Axios error response', () => {
  const error = {
    isAxiosError: true,
    message: 'Request failed with status code 400',
    response: {
      data: {
        name: 'bad_request',
        message: 'template dependency validation failed: dependency template is not reusable',
      },
    },
  }

  assert.equal(
    extractErrorMessage(error, 'Template save failed'),
    'template dependency validation failed: dependency template is not reusable',
  )
})
