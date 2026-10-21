// Holds an MSW handler open until release() is called, to test pending states.
export function gate() {
  let release = () => {}
  const wait = new Promise<void>((resolve) => (release = resolve))
  return { wait, release: () => release() }
}
