import { existsSync } from 'node:fs'
import { homedir } from 'node:os'
import path from 'node:path'

export function resolveBddPython(repoRoot: string): string {
  const configured = process.env.E2E_BDD_PYTHON?.trim()
  if (configured) {
    return configured
  }

  const candidates = [
    path.join(repoRoot, 'tests', 'bdd', '.venv', 'bin', 'python'),
    path.join(homedir(), '.dcs-bdd-venv', 'bin', 'python'),
    path.join(repoRoot, 'tests', 'bdd', '.venv', 'Scripts', 'python.exe'),
    path.join(homedir(), '.dcs-bdd-venv', 'Scripts', 'python.exe'),
  ]
  const detected = candidates.find((candidate) => existsSync(candidate))
  if (detected) {
    return detected
  }

  throw new Error(
    'BDD Python environment not found. Run `make -C tests/bdd setup_environment` from the repository root or set E2E_BDD_PYTHON. ' +
      `Checked: ${candidates.join(', ')}`,
  )
}
