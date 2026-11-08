package database

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/pgvector/pgvector-go"

	"main/internal/gen/table"
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

// textSearch is a typed query's SQL: a filter, and a score for ranking by it.
type textSearch struct {
	terms          []searchTerm
	configs        []string
	inTitle, inAny []pg.BoolExpression
	anyQueries     []string
	embedding      *pgvector.Vector
	// weights are each term's rarity (IDF), set by loadWeights.
	weights []float64
}

func termConfig(term searchTerm) (config, titleWeights, anyWeights string) {
	if term.exact {
		return "simple", "A", "AC"
	}

	return "english", "B", "BD"
}

// toTSQuery is to_tsquery in config; wrapped in SELECT so Postgres parses it once, not per row.
func toTSQuery(config, query string) pg.Expression {
	return pg.Raw("(SELECT to_tsquery('"+config+"', #query))", pg.RawArgs{"#query": query})
}

func tsMatch(config, query string) pg.BoolExpression {
	return pg.BoolExp(pg.CustomExpression(table.Jobs.SearchVector, pg.Token("@@"), toTSQuery(config, query)))
}

// cosineDistance is pgvector's <=> between col and v.
func cosineDistance(col pg.Expression, v pgvector.Vector) pg.FloatExpression {
	return pg.FloatExp(pg.CustomExpression(col, pg.Token("<=>"), arg(v, "vector")))
}

// num inlines a constant, as Postgres can't infer a type for a bare param in CASE or a projection.
func num(v float64) pg.FloatExpression {
	return pg.RawFloat(strconv.FormatFloat(v, 'g', -1, 64))
}

// newTextSearch builds the filter's tsqueries; embedding, when set, adds nearest jobs by meaning.
func newTextSearch(terms []searchTerm, embedding *pgvector.Vector) *textSearch {
	ts := &textSearch{terms: terms, embedding: embedding}

	for _, term := range terms {
		config, titleWeights, anyWeights := termConfig(term)

		ts.inTitle = append(ts.inTitle, tsMatch(config, term.tsqueryText(titleWeights)))
		ts.inAny = append(ts.inAny, tsMatch(config, term.tsqueryText(anyWeights)))
		ts.anyQueries = append(ts.anyQueries, term.tsqueryText(anyWeights))
		ts.configs = append(ts.configs, config)
	}

	return ts
}

// match passes jobs where half the terms hit the title or every term appears somewhere,
// plus the nearest jobs by embedding.
func (ts *textSearch) match() pg.BoolExpression {
	titleHits := make([]pg.Expression, len(ts.inTitle))

	for i, inTitle := range ts.inTitle {
		titleHits[i] = pg.CASE().WHEN(inTitle).THEN(num(1)).ELSE(num(0))
	}

	titleHitCount := pg.FloatExp(pg.CustomExpression(joinExpressions(titleHits, "+")...))
	match := titleHitCount.MUL(num(2)).GT_EQ(num(float64(len(ts.terms)))).OR(pg.AND(ts.inAny...))

	if ts.embedding == nil {
		return match
	}

	jobs := table.Jobs
	nearest := pg.SELECT(jobs.ID).FROM(jobs).
		WHERE(jobs.Embedding.IS_NOT_NULL()).
		ORDER_BY(cosineDistance(jobs.Embedding, *ts.embedding).ASC()).
		LIMIT(semanticCandidates)

	return match.OR(jobs.ID.IN(nearest))
}

// termWeightTTL bounds how stale cached rarity weights get when another process wrote jobs.
const termWeightTTL = 15 * time.Minute

const maxCachedTermWeights = 1000

type cachedWeight struct {
	weight  float64
	expires time.Time
}

var termWeights = struct {
	sync.Mutex
	entries map[string]cachedWeight
}{entries: map[string]cachedWeight{}}

// forgetTermWeights drops cached rarity weights after jobs change.
func forgetTermWeights() {
	termWeights.Lock()
	clear(termWeights.entries)
	termWeights.Unlock()
}

// loadWeights sets each term's rarity, ln(1 + jobs / (1 + jobs matching)), so "intern" outweighs
// "engineer". Uncached terms are counted in one scan.
func (ts *textSearch) loadWeights(ctx context.Context) error {
	ts.weights = make([]float64, len(ts.terms))
	keys := make([]string, len(ts.terms))
	var missing []int

	termWeights.Lock()
	for i := range ts.terms {
		keys[i] = ts.configs[i] + "|" + ts.anyQueries[i]
		if entry, ok := termWeights.entries[keys[i]]; ok && time.Now().Before(entry.expires) {
			ts.weights[i] = entry.weight
		} else {
			missing = append(missing, i)
		}
	}
	termWeights.Unlock()

	if len(missing) == 0 {
		return nil
	}

	counts := []pg.Projection{pg.COUNT(pg.STAR).AS("total")}
	for n, i := range missing {
		counts = append(counts, pg.IntExp(pg.CustomExpression(pg.Token("count(*) FILTER (WHERE"), ts.inAny[i], pg.Token(")"))).
			AS(fmt.Sprintf("m%d", n)))
	}

	query, args := pg.SELECT(counts[0], counts[1:]...).FROM(table.Jobs).Sql()
	values := make([]int64, len(counts))
	dest := make([]any, len(values))
	for i := range values {
		dest[i] = &values[i]
	}

	if err := db().QueryRowContext(ctx, query, args...).Scan(dest...); err != nil {
		return fmt.Errorf("count search terms: %w", err)
	}

	termWeights.Lock()
	defer termWeights.Unlock()

	if len(termWeights.entries) >= maxCachedTermWeights {
		clear(termWeights.entries)
	}

	for n, i := range missing {
		ts.weights[i] = math.Log(1 + float64(values[0])/float64(1+values[n+1]))
		termWeights.entries[keys[i]] = cachedWeight{weight: ts.weights[i], expires: time.Now().Add(termWeightTTL)}
	}

	return nil
}

// score is 0..1 keyword relevance with terms weighted by loadWeights, plus semanticWeight times
// query-to-job similarity when there's an embedding.
func (ts *textSearch) score() pg.FloatExpression {
	jobs := table.Jobs
	scores := make([]pg.Expression, len(ts.terms))
	total := 0.0

	for i, term := range ts.terms {
		inExactTitle := ts.inTitle[i]

		if !term.exact {
			inExactTitle = tsMatch("simple", term.tsqueryText("A"))
		}

		total += ts.weights[i]
		scores[i] = num(ts.weights[i]).MUL(pg.FloatExp(pg.CASE().
			WHEN(inExactTitle).THEN(num(exactTitleHitScore)).
			WHEN(ts.inTitle[i]).THEN(num(titleHitScore)).
			WHEN(ts.inAny[i]).THEN(num(descriptionHitScore)).
			ELSE(num(0))))
	}

	// No jobs means every weight is 0; any divisor works since no row is scored.
	if total == 0 {
		total = 1
	}

	score := pg.FloatExp(pg.CustomExpression(joinExpressions(scores, "+")...)).DIV(num(total))

	if ts.embedding == nil {
		return score
	}

	similarity := pg.FloatExp(pg.COALESCE(num(1).SUB(cosineDistance(jobs.Embedding, *ts.embedding)), num(0)))

	return score.ADD(num(semanticWeight).MUL(similarity))
}

// joinExpressions interleaves exprs with op, for a CustomExpression like "a + b + c", parenthesized.
func joinExpressions(exprs []pg.Expression, op string) []pg.Serializer {
	parts := []pg.Serializer{pg.Token("(")}

	for i, expr := range exprs {
		if i > 0 {
			parts = append(parts, pg.Token(op))
		}

		parts = append(parts, expr)
	}

	return append(parts, pg.Token(")"))
}
