import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { describe, it } from 'node:test'
import { resolveBddPython } from '../../e2e/bdd-python.ts'

function withoutConfiguredPython<T>(run: () => T): T {
  const configured = process.env.E2E_BDD_PYTHON
  delete process.env.E2E_BDD_PYTHON
  try {
    return run()
  } finally {
    if (configured === undefined) {
      delete process.env.E2E_BDD_PYTHON
    } else {
      process.env.E2E_BDD_PYTHON = configured
    }
  }
}

void describe('resolveBddPython', () => {
  void it('prefers the explicitly configured interpreter', () => {
    const configured = process.env.E2E_BDD_PYTHON
    process.env.E2E_BDD_PYTHON = '/configured/python'
    try {
      assert.equal(resolveBddPython('/repo'), '/configured/python')
    } finally {
      if (configured === undefined) {
        delete process.env.E2E_BDD_PYTHON
      } else {
        process.env.E2E_BDD_PYTHON = configured
      }
    }
  })

  void it('detects the repository-local environment created on native Linux', () => {
    const repoRoot = mkdtempSync(path.join(tmpdir(), 'dcs-bdd-python-'))
    const python = path.join(repoRoot, 'tests', 'bdd', '.venv', 'bin', 'python')
    mkdirSync(path.dirname(python), { recursive: true })
    writeFileSync(python, '')
    try {
      withoutConfiguredPython(() => assert.equal(resolveBddPython(repoRoot), python))
    } finally {
      rmSync(repoRoot, { recursive: true, force: true })
    }
  })
})
