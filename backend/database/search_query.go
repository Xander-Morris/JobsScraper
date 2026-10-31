package database

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pgvector/pgvector-go"
)

// searchTerm is one typed word plus the words and phrases that count as a match for it.
type searchTerm struct {
	variants []string
	// exact matches unstemmed lexemes, so "intern" doesn't hit "internal" or "international".
	exact bool
}

const maxSearchTerms = 8

// Title hits outscore description hits so "intern" in a title beats "intern" buried in a posting.
// Unstemmed title hits beat stemmed ones so "accountant" prefers "Accountant" over "Account Executive".
const (
	exactTitleHitScore  = 1.0
	titleHitScore       = 0.7
	descriptionHitScore = 0.3
	// semanticWeight scales query-to-job cosine similarity when embeddings exist.
	semanticWeight = 0.5
	// semanticCandidates is how many nearest jobs by embedding join the keyword matches.
	semanticCandidates = 50
)

var searchWordPattern = regexp.MustCompile(`[a-z0-9]+`)

// Joins compound words so "front end", "front-end" and "frontend" all read as one term.
var compoundPattern = regexp.MustCompile(`\b(front|back|full)[\s-]+(end|stack)\b`)

var (
	internTerm    = searchTerm{variants: []string{"intern", "interns", "internship", "internships", "co-op"}, exact: true}
	devTerm       = searchTerm{variants: []string{"dev", "developer", "engineer", "programmer"}}
	sweTerm       = searchTerm{variants: []string{"software engineer", "software developer", "swe", "sde"}}
	aiTerm        = searchTerm{variants: []string{"ai", "artificial intelligence", "machine learning", "ml"}}
	mlTerm        = searchTerm{variants: []string{"ml", "machine learning"}}
	pmTerm        = searchTerm{variants: []string{"pm", "product manager", "project manager"}}
	seniorTerm    = searchTerm{variants: []string{"senior", "sr"}}
	juniorTerm    = searchTerm{variants: []string{"junior", "jr", "entry level", "new grad"}}
	managerTerm   = searchTerm{variants: []string{"manager", "mgr"}}
	jsTerm        = searchTerm{variants: []string{"javascript", "js"}}
	tsTerm        = searchTerm{variants: []string{"typescript", "ts"}}
	goTerm        = searchTerm{variants: []string{"go", "golang"}}
	k8sTerm       = searchTerm{variants: []string{"kubernetes", "k8s"}}
	qaTerm        = searchTerm{variants: []string{"qa", "quality assurance", "test"}}
	sreTerm       = searchTerm{variants: []string{"sre", "site reliability"}}
	uxTerm        = searchTerm{variants: []string{"ux", "user experience"}}
	uiTerm        = searchTerm{variants: []string{"ui", "user interface", "frontend"}}
	hrTerm        = searchTerm{variants: []string{"hr", "human resources"}}
	itTerm        = searchTerm{variants: []string{"it", "information technology"}, exact: true}
	webTerm       = searchTerm{variants: []string{"web", "website", "frontend", "front end", "fullstack", "full stack"}}
	frontendTerm  = searchTerm{variants: []string{"frontend", "front end"}}
	backendTerm   = searchTerm{variants: []string{"backend", "back end"}}
	fullstackTerm = searchTerm{variants: []string{"fullstack", "full stack"}}
	dataSciTerm   = searchTerm{variants: []string{"data scientist", "data science", "ds"}}
)

var searchSynonyms = map[string]searchTerm{
	"intern": internTerm, "interns": internTerm, "internship": internTerm, "internships": internTerm,
	"coop": internTerm,
	"dev":  devTerm, "devs": devTerm, "developer": devTerm, "developers": devTerm,
	"engineer": devTerm, "engineers": devTerm, "eng": devTerm, "programmer": devTerm,
	"swe": sweTerm, "sde": sweTerm,
	"ai": aiTerm, "ml": mlTerm,
	"pm":     pmTerm,
	"senior": seniorTerm, "sr": seniorTerm,
	"junior": juniorTerm, "jr": juniorTerm,
	"manager": managerTerm, "mgr": managerTerm,
	"javascript": jsTerm, "js": jsTerm,
	"typescript": tsTerm, "ts": tsTerm,
	"go": goTerm, "golang": goTerm,
	"kubernetes": k8sTerm, "k8s": k8sTerm,
	"qa":  qaTerm,
	"sre": sreTerm,
	"ux":  uxTerm, "ui": uiTerm,
	"hr":  hrTerm,
	"it":  itTerm,
	"web": webTerm, "website": webTerm,
	"frontend":  frontendTerm,
	"backend":   backendTerm,
	"fullstack": fullstackTerm,
	"ds":        dataSciTerm,
}

// Postgres's english stopwords, which to_tsquery('english') drops; a term made of one would never match.
var searchStopwords = toSet(`i me my myself we our ours ourselves you your yours yourself yourselves he him his
	himself she her hers herself it its itself they them their theirs themselves what which who whom this that these
	those am is are was were be been being have has had having do does did doing a an the and but if or because as
	until while of at by for with about against between into through during before after above below to from up down
	in out on off over under again further then once here there when where why how all any both each few more most
	other some such no nor not only own same so than too very s t can will just don should now job jobs role roles
	position positions`)

func toSet(words string) map[string]bool {
	set := map[string]bool{}

	for _, w := range strings.Fields(words) {
		set[w] = true
	}

	return set
}

// parseSearchQuery splits a typed query into deduped terms, expanding abbreviations and synonyms.
func parseSearchQuery(query string) []searchTerm {
	normalized := compoundPattern.ReplaceAllString(strings.ToLower(query), "$1$2")

	var terms []searchTerm
	seen := map[string]bool{}

	for _, word := range searchWordPattern.FindAllString(normalized, -1) {
		term, ok := searchSynonyms[word]

		if !ok {
			if searchStopwords[word] {
				continue
			}

			term = searchTerm{variants: []string{word}}
		}

		key := strings.Join(term.variants, "|")

		if seen[key] {
			continue
		}

		seen[key] = true
		terms = append(terms, term)

		if len(terms) == maxSearchTerms {
			break
		}
	}

	return terms
}

// tsqueryText ORs the term's variants, each a phrase of words restricted to weights.
func (t searchTerm) tsqueryText(weights string) string {
	alternatives := make([]string, 0, len(t.variants))

	for _, variant := range t.variants {
		words := searchWordPattern.FindAllString(variant, -1)

		for i, w := range words {
			words[i] = w + ":" + weights
		}

		alternatives = append(alternatives, "("+strings.Join(words, " <-> ")+")")
	}

	return strings.Join(alternatives, " | ")
}

// textSearch is a typed query's SQL. Its filter's args go in the WHERE args; score's are
// appended only when ranking uses it, since Postgres rejects params it never sees.
type textSearch struct {
	terms          []searchTerm
	configs        []string
	inTitle, inAny []string
	anyArgs        []int
	embeddingArg   int
}

func termConfig(term searchTerm) (config, titleWeights, anyWeights string) {
	if term.exact {
		return "simple", "A", "AC"
	}

	return "english", "B", "BD"
}

func tsMatch(config string, arg int) string {
	return fmt.Sprintf("j.search_vector @@ (SELECT to_tsquery('%s', $%d))", config, arg)
}

// newTextSearch appends the filter's tsquery args; embedding, when set, adds nearest jobs by meaning.
func newTextSearch(args *[]any, terms []searchTerm, embedding *pgvector.Vector) *textSearch {
	ts := &textSearch{terms: terms}

	for _, term := range terms {
		config, titleWeights, anyWeights := termConfig(term)

		*args = append(*args, term.tsqueryText(titleWeights))
		ts.inTitle = append(ts.inTitle, tsMatch(config, len(*args)))

		*args = append(*args, term.tsqueryText(anyWeights))
		ts.inAny = append(ts.inAny, tsMatch(config, len(*args)))
		ts.anyArgs = append(ts.anyArgs, len(*args))
		ts.configs = append(ts.configs, config)
	}

	if embedding != nil {
		*args = append(*args, *embedding)
		ts.embeddingArg = len(*args)
	}

	return ts
}

// match passes jobs where half the terms hit the title or every term appears somewhere,
// plus the nearest jobs by embedding.
func (ts *textSearch) match() string {
	titleHits := make([]string, len(ts.terms))

	for i, inTitle := range ts.inTitle {
		titleHits[i] = fmt.Sprintf("CASE WHEN %s THEN 1 ELSE 0 END", inTitle)
	}

	match := fmt.Sprintf("((%s) * 2 >= %d OR (%s))",
		strings.Join(titleHits, " + "), len(ts.terms), strings.Join(ts.inAny, " AND "))

	if ts.embeddingArg == 0 {
		return match
	}

	return fmt.Sprintf(`(%s OR j.id IN (SELECT id FROM jobs WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $%d LIMIT %d))`, match, ts.embeddingArg, semanticCandidates)
}

// score is 0..1 keyword relevance with terms weighted by rarity (IDF), so "intern" outweighs
// "engineer", plus semanticWeight times query-to-job similarity when there's an embedding.
func (ts *textSearch) score(args *[]any) string {
	scores := make([]string, len(ts.terms))
	weights := make([]string, len(ts.terms))

	for i, term := range ts.terms {
		inExactTitle := ts.inTitle[i]

		if !term.exact {
			*args = append(*args, term.tsqueryText("A"))
			inExactTitle = tsMatch("simple", len(*args))
		}

		weights[i] = fmt.Sprintf(`(SELECT ln(1 + (SELECT count(*) FROM jobs)::float / (1 + count(*)))
			FROM jobs WHERE search_vector @@ to_tsquery('%s', $%d))`, ts.configs[i], ts.anyArgs[i])
		scores[i] = fmt.Sprintf("%s * CASE WHEN %s THEN %g WHEN %s THEN %g WHEN %s THEN %g ELSE 0 END",
			weights[i], inExactTitle, exactTitleHitScore, ts.inTitle[i], titleHitScore, ts.inAny[i], descriptionHitScore)
	}

	score := fmt.Sprintf("((%s) / (%s))", strings.Join(scores, " + "), strings.Join(weights, " + "))

	if ts.embeddingArg == 0 {
		return score
	}

	return fmt.Sprintf("(%s + %g * coalesce(1 - (j.embedding <=> $%d), 0))", score, semanticWeight, ts.embeddingArg)
}
