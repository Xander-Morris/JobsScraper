function applyTheme(isDark: boolean) {
  document.documentElement.classList.toggle('dark', isDark)
}

export function initTheme() {
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  applyTheme(media.matches)
  media.addEventListener('change', (e) => applyTheme(e.matches))
}
