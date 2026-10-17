package crawler

import "regexp"

// workdayJobPage matches Workday's job detail pages, skipping its search and login pages.
var workdayJobPage = regexp.MustCompile(`/job/`)

func workday(siteURL, company string) Seed {
	return Seed{URL: siteURL, Company: company, Include: workdayJobPage}
}

// seeds are the career sites crawled each scrape. Any site whose job pages embed
// schema.org JobPosting JSON-LD works; Workday sites list every posting in a sitemap.
var seeds = []Seed{
	workday("https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite", "NVIDIA"),
	workday("https://salesforce.wd12.myworkdayjobs.com/External_Career_Site", "Salesforce"),
	workday("https://salesforce.wd12.myworkdayjobs.com/Futureforce_NewGradRoles", "Salesforce"),
	workday("https://adobe.wd5.myworkdayjobs.com/external_experienced", "Adobe"),
	workday("https://intel.wd1.myworkdayjobs.com/External", "Intel"),
	workday("https://mastercard.wd1.myworkdayjobs.com/CorporateCareers", "Mastercard"),
	workday("https://mastercard.wd1.myworkdayjobs.com/Campus", "Mastercard"),
	workday("https://cisco.wd5.myworkdayjobs.com/Cisco_Careers", "Cisco"),
	workday("https://redhat.wd5.myworkdayjobs.com/Jobs", "Red Hat"),
	workday("https://target.wd5.myworkdayjobs.com/targetcareers", "Target"),
	workday("https://hp.wd5.myworkdayjobs.com/ExternalCareerSite", "HP"),
}
