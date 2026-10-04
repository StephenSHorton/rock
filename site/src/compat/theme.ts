/** Host fallback when `mokei` cannot be installed (private git package). */
export type ThemeDefinition = {
  id: string
  name: string
  description: string
  dataTheme: string
}

export function applyTheme(theme: ThemeDefinition | string) {
  const id = typeof theme === 'string' ? theme : theme.dataTheme
  document.documentElement.dataset.theme = id
}
