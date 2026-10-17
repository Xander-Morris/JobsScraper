package jobs

import "testing"

func TestYearlySalary(t *testing.T) {
	tests := []struct {
		name             string
		low, high        float64
		period, currency string
		wantMin, wantMax *int
	}{
		{"yearly usd", 60000, 90000, "yearly", "USD", intPtr(60000), intPtr(90000)},
		{"no period or currency means yearly usd", 60000, 90000, "", "", intPtr(60000), intPtr(90000)},
		{"hourly annualized", 34.32, 42.13, "hourly", "USD", intPtr(71386), intPtr(87630)},
		{"ashby interval", 211400, 290600, "1 YEAR", "USD", intPtr(211400), intPtr(290600)},
		{"lever hourly interval", 50, 60, "per-hour-wage", "usd", intPtr(104000), intPtr(124800)},
		{"monthly", 5000, 6000, "monthly", "USD", intPtr(60000), intPtr(72000)},
		{"other currency skipped", 140000, 165000, "yearly", "GBP", nil, nil},
		{"unknown period skipped", 1000, 2000, "one-time", "USD", nil, nil},
		{"missing max", 80000, 0, "yearly", "USD", intPtr(80000), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := YearlySalary(tt.low, tt.high, tt.period, tt.currency)

			if !intPtrEqual(min, tt.wantMin) || !intPtrEqual(max, tt.wantMax) {
				t.Errorf("YearlySalary = %v, %v, want %v, %v", deref(min), deref(max), deref(tt.wantMin), deref(tt.wantMax))
			}
		})
	}
}

func TestParseSalaryTextField(t *testing.T) {
	tests := []struct {
		text     string
		min, max *int
	}{
		{"$100k - $150k", intPtr(100000), intPtr(150000)},
		{"$80,000 – $120,000 USD", intPtr(80000), intPtr(120000)},
		{"$120K-$90K", intPtr(90000), intPtr(120000)},
		{"$95000", intPtr(95000), intPtr(95000)},
		{"$100-150k", intPtr(100000), intPtr(150000)},
		{"50k-70k USD", intPtr(50000), intPtr(70000)},
		{"$40 - $60 per hour", intPtr(83200), intPtr(124800)},
		{"$50/hr", intPtr(104000), intPtr(104000)},
		{"$34.32 – $42.13 per hour", intPtr(71386), intPtr(87630)},
		{"$60,000 - $97,000/year", intPtr(60000), intPtr(97000)},
		{"$5,000 - $6,000 a month", intPtr(60000), intPtr(72000)},
		{"€40k - €60k", nil, nil},
		{"CA$90,000 – CA$110,000", nil, nil},
		{"$90,000 - $110,000 CAD", nil, nil},
		{"$5 million", nil, nil},
		{"50", nil, nil},
		{"Competitive", nil, nil},
		{"", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			min, max := ParseSalaryText(tt.text, false)

			if !intPtrEqual(min, tt.min) || !intPtrEqual(max, tt.max) {
				t.Errorf("ParseSalaryText(%q) = %v, %v, want %v, %v", tt.text, deref(min), deref(max), deref(tt.min), deref(tt.max))
			}
		})
	}
}

func TestParseSalaryTextDescription(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		min, max *int
	}{
		{"greenhouse pay range", "The annual base salary range for this position is $150,000—$200,000 USD.",
			intPtr(150000), intPtr(200000)},
		{"geo tiers combine", "Salary ranges: San Francisco $180,000 - $220,000; Remote US $150,000 - $190,000",
			intPtr(150000), intPtr(220000)},
		{"hourly pay word before range", "Hourly pay range: $25 - $30", intPtr(52000), intPtr(62400)},
		{"between and", "Compensation is between $90,000 and $110,000.", intPtr(90000), intPtr(110000)},
		{"single amount skipped", "Base salary: $120,000", nil, nil},
		{"401k is not a range end", "Base salary $100,000 and 401k matching", nil, nil},
		{"no pay word", "We offer $10,000 - $20,000 in relocation support.", nil, nil},
		{"millions skipped", "businesses typically processing $20M–$50M in annual payment volume", nil, nil},
		{"stipend skipped", "Enjoy a $5,000 learning stipend.", nil, nil},
		{"foreign dollars skipped", "Salary range: CA$90,000 - CA$110,000", nil, nil},
		{"monthly perk doesn't change salary period", "A monthly wellness stipend. Salary: $120,000 - $150,000",
			intPtr(120000), intPtr(150000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := ParseSalaryText(tt.text, true)

			if !intPtrEqual(min, tt.min) || !intPtrEqual(max, tt.max) {
				t.Errorf("ParseSalaryText(%q) = %v, %v, want %v, %v", tt.text, deref(min), deref(max), deref(tt.min), deref(tt.max))
			}
		})
	}
}

func intPtrEqual(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func deref(p *int) any {
	if p == nil {
		return nil
	}

	return *p
}
