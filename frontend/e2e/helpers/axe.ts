/**
 * AP-075 a11y scanning helper.
 * Axe violations block CI; incomplete results do not, but are flagged.
 */
import AxeBuilder from '@axe-core/playwright'
import type { Page } from '@playwright/test'

export interface AxeResult {
  violations: number
  incomplete: number
  passes: number
  details: string
}

/**
 * Scan a page (or element scoped by selector) with axe-core.
 * Returns structured results for assertion.
 */
export async function scanA11y(page: Page, selector?: string): Promise<AxeResult> {
  const builder = new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'best-practice'])
    .options({
      runOnly: {
        type: 'tag',
        values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'best-practice'],
      },
    })

  if (selector) {
    builder.include(selector)
  }

  const results = await builder.analyze()

  const details = results.violations
    .map(
      (v) =>
        `[${v.id}] ${v.help} — ${v.nodes.length} node(s): ${v.nodes
          .map((n) => n.target.join(' > '))
          .join('; ')}`,
    )
    .join('\n')

  return {
    violations: results.violations.length,
    incomplete: results.incomplete.length,
    passes: results.passes.length,
    details,
  }
}
