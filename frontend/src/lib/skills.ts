// Skill matching between a posting and the skills pulled off the active
// resume. Both sides are noisy free text, so everything is normalized to
// lowercase tokens before comparing, so "Node.js" and "node js" are one skill.
// `+` and `#` survive normalization so "c++" and "c#" stay distinct from "c".
function normalize(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9+#]+/g, ' ')
    .trim()
}

export interface SkillMatch {
  /** Resume skills this posting asks for; worth leading with. */
  matched: string[]
  /** Posting tags with no matching resume skill; the gaps to address. */
  missing: string[]
}

// matchSkills compares the posting's tags and description against the resume
// skills. Matching is whole-token only. That prevents something like "go" and "mongodb" matching together.
export function matchSkills(
  tags: string[],
  description: string,
  resumeSkills: string[] | null | undefined
): SkillMatch {
  const skills = (resumeSkills ?? []).filter((skill) => skill.trim().length > 0)
  if (skills.length === 0) return { matched: [], missing: [] }

  const posting = ` ${normalize([...tags, description].join(' '))} `
  const seen = new Set<string>()
  const matched: string[] = []

  for (const skill of skills) {
    const needle = normalize(skill)
    if (needle.length < 2 || seen.has(needle)) continue
    if (!posting.includes(` ${needle} `)) continue
    seen.add(needle)
    matched.push(skill)
  }

  const resume = ` ${skills.map(normalize).join(' ')} `
  const missing = tags.filter((tag) => {
    const needle = normalize(tag)
    return needle.length >= 2 && !resume.includes(` ${needle} `)
  })

  return { matched, missing }
}
