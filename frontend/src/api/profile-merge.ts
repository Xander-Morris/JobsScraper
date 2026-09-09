import {
  addEducation,
  addSkill,
  addWorkExperience,
  addWorkExperienceBullet,
  deleteWorkExperienceBullet,
  updateEducation,
  updateProfile,
  updateWorkExperience,
  useProfileMutation,
} from './profile'
import type { Profile, ResumeExtraction } from './schemas'

const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/

export interface ApplyResumeExtractionResult {
  updatedBasicInfo: boolean
  addedEducation: number
  updatedEducation: number
  addedSkills: number
  addedWorkExperience: number
  updatedWorkExperience: number
}

function asDate(value: string): string | undefined {
  return isoDatePattern.test(value) ? value : undefined
}

async function mergeBasicInfo(token: string, profile: Profile, extraction: ResumeExtraction): Promise<boolean> {
  const nextName = profile.name || extraction.full_name
  const nextLinkedIn = profile.linked_in || extraction.linked_in
  const nextGithub = profile.github || extraction.github
  const nextPortfolio = profile.portfolio || extraction.portfolio

  const changed =
    nextName !== profile.name ||
    nextLinkedIn !== profile.linked_in ||
    nextGithub !== profile.github ||
    nextPortfolio !== profile.portfolio

  if (changed) {
    await updateProfile(token, {
      name: nextName,
      address: profile.address,
      linked_in: nextLinkedIn,
      github: nextGithub,
      portfolio: nextPortfolio,
      email_notifications: profile.email_notifications,
    })
  }

  return changed
}

async function mergeSkills(token: string, profile: Profile, extraction: ResumeExtraction): Promise<number> {
  const existingSkills = new Set((profile.skills ?? []).map((s) => s.skill.trim().toLowerCase()))
  let added = 0

  for (const raw of extraction.skills ?? []) {
    const skill = raw.trim()
    if (!skill || existingSkills.has(skill.toLowerCase())) continue
    existingSkills.add(skill.toLowerCase())
    await addSkill(token, { skill })
    added++
  }

  return added
}

async function mergeEducation(
  token: string,
  profile: Profile,
  extraction: ResumeExtraction,
): Promise<{ added: number; updated: number }> {
  const educationByKey = new Map(
    (profile.education ?? []).map((e) => [
      `${e.school_name.trim().toLowerCase()}|${e.major.trim().toLowerCase()}|${e.degree.trim().toLowerCase()}`,
      e,
    ]),
  )
  let added = 0
  let updated = 0

  for (const entry of extraction.education ?? []) {
    const schoolName = entry.school_name.trim()
    const major = entry.major.trim()
    const degree = entry.degree.trim()
    if (!schoolName || !major || !degree) continue

    const key = `${schoolName.toLowerCase()}|${major.toLowerCase()}|${degree.toLowerCase()}`
    const existing = educationByKey.get(key)
    const startDate = asDate(entry.start_date) ?? existing?.start_date ?? undefined
    const endDate = asDate(entry.end_date) ?? existing?.end_date ?? undefined

    if (existing) {
      if (startDate === (existing.start_date ?? undefined) && endDate === (existing.end_date ?? undefined)) continue

      await updateEducation(token, existing.id, {
        school_name: schoolName,
        major,
        degree,
        gpa: existing.gpa,
        start_date: startDate,
        end_date: endDate,
      })
      updated++
    } else {
      await addEducation(token, { school_name: schoolName, major, degree, start_date: startDate, end_date: endDate })
      added++
    }
  }

  return { added, updated }
}

async function mergeWorkExperience(
  token: string,
  profile: Profile,
  extraction: ResumeExtraction,
): Promise<{ added: number; updated: number }> {
  const workExperienceByKey = new Map(
    (profile.work_experience ?? []).map((w) => [`${w.company.trim().toLowerCase()}|${w.job_title.trim().toLowerCase()}`, w]),
  )
  let added = 0
  let updated = 0

  for (const entry of extraction.work_experience ?? []) {
    const company = entry.company.trim()
    const jobTitle = entry.job_title.trim()
    if (!company || !jobTitle) continue

    const key = `${company.toLowerCase()}|${jobTitle.toLowerCase()}`
    const existing = workExperienceByKey.get(key)
    const extractedBullets = [...new Set((entry.bullets ?? []).map((b) => b.trim()).filter(Boolean))]

    if (existing) {
      const location = entry.location.trim() || existing.location || ''
      const startDate = asDate(entry.start_date) ?? existing.start_date ?? undefined
      const endDate = asDate(entry.end_date) ?? existing.end_date ?? undefined
      const fieldsChanged =
        location !== (existing.location ?? '') ||
        startDate !== (existing.start_date ?? undefined) ||
        endDate !== (existing.end_date ?? undefined)

      const existingBullets = existing.bullets ?? []
      const existingBulletTexts = new Set(existingBullets.map((b) => b.bullet.trim()))
      const extractedBulletTexts = new Set(extractedBullets)
      const bulletsToAdd = extractedBullets.filter((b) => !existingBulletTexts.has(b))
      const bulletsToRemove = existingBullets.filter((b) => !extractedBulletTexts.has(b.bullet.trim()))

      if (!fieldsChanged && bulletsToAdd.length === 0 && bulletsToRemove.length === 0) continue

      if (fieldsChanged) {
        await updateWorkExperience(token, existing.id, {
          company,
          job_title: jobTitle,
          job_type: existing.job_type,
          location,
          start_date: startDate,
          end_date: endDate,
        })
      }

      for (const bullet of bulletsToAdd) {
        await addWorkExperienceBullet(token, existing.id, { bullet })
      }
      for (const bullet of bulletsToRemove) {
        await deleteWorkExperienceBullet(token, existing.id, bullet.id)
      }

      updated++
    } else {
      const { id } = await addWorkExperience(token, {
        company,
        job_title: jobTitle,
        job_type: 'unknown',
        location: entry.location,
        start_date: asDate(entry.start_date),
        end_date: asDate(entry.end_date),
      })

      for (const bullet of extractedBullets) {
        await addWorkExperienceBullet(token, id, { bullet })
      }

      added++
    }
  }

  return { added, updated }
}

export async function applyResumeExtractionToProfile(
  token: string,
  profile: Profile,
  extraction: ResumeExtraction,
): Promise<ApplyResumeExtractionResult> {
  const updatedBasicInfo = await mergeBasicInfo(token, profile, extraction)
  const addedSkills = await mergeSkills(token, profile, extraction)
  const { added: addedEducation, updated: updatedEducation } = await mergeEducation(token, profile, extraction)
  const { added: addedWorkExperience, updated: updatedWorkExperience } = await mergeWorkExperience(
    token,
    profile,
    extraction,
  )

  return { updatedBasicInfo, addedEducation, updatedEducation, addedSkills, addedWorkExperience, updatedWorkExperience }
}

export function useApplyResumeExtractionMutation(token: string | null) {
  return useProfileMutation(({ profile, extraction }: { profile: Profile; extraction: ResumeExtraction }) =>
    applyResumeExtractionToProfile(token!, profile, extraction),
  )
}
