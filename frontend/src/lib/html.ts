export function decodeHtmlEntities(input: string): string {
  const textarea = document.createElement('textarea')
  textarea.innerHTML = input
  return textarea.value
}
