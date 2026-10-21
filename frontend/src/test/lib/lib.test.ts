import { describe, expect, it, vi } from 'vitest'
import downloadBlob from '@/src/lib/download-blob'
import { decodeHtmlEntities } from '@/src/lib/html'
import { jobSearchSchema, lastJobSearch, rememberJobSearch } from '@/src/lib/job-search'
import { matchSkills } from '@/src/lib/skills'
import { newPasswordSchema } from '@/src/lib/password'
import slugify from '@/src/lib/slugify'
import { cn, errorMessage } from '@/src/lib/utils'

describe('matchSkills', () => {
  it('matches whole tokens only', () => {
    const result = matchSkills(['mongodb', 'react'], 'We use MongoDB and React.', ['Go', 'React'])

    expect(result.matched).toEqual(['React'])
    expect(result.missing).toEqual(['mongodb'])
  })

  it('normalizes punctuation and case', () => {
    expect(matchSkills([], 'Experience with node js required', ['Node.js']).matched).toEqual(['Node.js'])
    expect(matchSkills(['TypeScript'], '', ['typescript']).missing).toEqual([])
  })

  it('keeps c++ and c# distinct from c', () => {
    const result = matchSkills(['c++', 'c#'], 'We write C++ and C#.', ['C', 'C++'])

    expect(result.matched).toEqual(['C++'])
    expect(result.missing).toEqual(['c#'])
  })

  it('dedupes skills and skips one-letter needles', () => {
    const result = matchSkills(['r'], 'go go go', ['Go', 'go', 'R'])

    expect(result.matched).toEqual(['Go'])
    expect(result.missing).toEqual([])
  })

  it('returns nothing without resume skills', () => {
    expect(matchSkills(['go'], 'go', null)).toEqual({ matched: [], missing: [] })
    expect(matchSkills(['go'], 'go', undefined)).toEqual({ matched: [], missing: [] })
    expect(matchSkills(['go'], 'go', ['  ', ''])).toEqual({ matched: [], missing: [] })
  })
})

describe('slugify', () => {
  it.each([
    ['Senior Go Engineer', 'senior-go-engineer'],
    ['  C++ / Rust!! ', 'c-rust'],
    ['Ünïcode Jöb', 'n-code-j-b'],
    ['---', 'job'],
    ['', 'job']
  ])('%s -> %s', (input, want) => {
    expect(slugify(input)).toBe(want)
  })
})

describe('decodeHtmlEntities', () => {
  it('decodes named and numeric entities', () => {
    expect(decodeHtmlEntities('R&amp;D &lt;team&gt; &#39;quoted&#39; caf&eacute;')).toBe("R&D <team> 'quoted' café")
  })

  it('leaves plain text alone and does not run markup', () => {
    expect(decodeHtmlEntities('plain')).toBe('plain')
    expect(decodeHtmlEntities('<img src=x onerror=alert(1)>')).toBe('<img src=x onerror=alert(1)>')
  })
})

describe('cn', () => {
  it('merges conflicting tailwind classes and drops falsy ones', () => {
    const hidden = false
    expect(cn('p-2', hidden && 'hidden', 'p-4', undefined, ['text-sm'])).toBe('p-4 text-sm')
  })
})

describe('errorMessage', () => {
  it.each([
    [null, null],
    [undefined, null],
    [new Error('boom'), 'boom'],
    ['string error', 'Something went wrong'],
    [{ message: 'not an Error' }, 'Something went wrong']
  ])('%s -> %s', (input, want) => {
    expect(errorMessage(input)).toBe(want)
  })
})

describe('job search memory', () => {
  it('round trips a valid search', () => {
    rememberJobSearch({ q: 'go', tags: ['react'], page: 2, workplaceType: 'remote' })
    expect(lastJobSearch()).toEqual({ q: 'go', tags: ['react'], page: 2, workplaceType: 'remote' })
  })

  it('returns an empty search when nothing is stored or it is corrupt', () => {
    expect(lastJobSearch()).toEqual({})

    sessionStorage.setItem('jobs-last-search', '{not json')
    expect(lastJobSearch()).toEqual({})

    sessionStorage.setItem('jobs-last-search', JSON.stringify({ workplaceType: 'moon' }))
    expect(lastJobSearch()).toEqual({})
  })

  it('survives blocked storage', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })

    expect(() => rememberJobSearch({ q: 'x' })).not.toThrow()
    expect(lastJobSearch()).toEqual({})
  })

  it('schema rejects unknown enum values', () => {
    expect(jobSearchSchema.safeParse({ sort: 'random' }).success).toBe(false)
    expect(jobSearchSchema.safeParse({ datePosted: '3d', jobType: 'intern' }).success).toBe(true)
  })
})

describe('downloadBlob', () => {
  it('clicks a temporary link to the blob and revokes it', () => {
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:resume')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      expect(this.href).toBe('blob:resume')
      expect(this.download).toBe('resume.pdf')
    })

    const blob = new Blob(['x'])
    downloadBlob(blob, 'resume.pdf')

    expect(create).toHaveBeenCalledWith(blob)
    expect(click).toHaveBeenCalledOnce()
    expect(revoke).toHaveBeenCalledWith('blob:resume')
  })
})

describe('newPasswordSchema', () => {
  const error = (p: string) => newPasswordSchema.safeParse(p).error?.issues[0].message

  it('requires 8 characters and caps at 72 bytes', () => {
    expect(error('short')).toBe('At least 8 characters')
    expect(error('p'.repeat(72))).toBeUndefined()
    expect(error('p'.repeat(73))).toBe('Too long (72 bytes max)')
    // 37 two-byte characters is 74 bytes.
    expect(error('é'.repeat(37))).toBe('Too long (72 bytes max)')
  })
})
