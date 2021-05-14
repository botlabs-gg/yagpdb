package covidstats

//problem here is, API does not state which values could change to floats : )
type coronaWorldWideStruct struct {
	Updated                int64
	Country                string
	State                  string
	CountryInfo            countryInfoStruct
	Cases                  int64
	TodayCases             int64
	Deaths                 int64
	TodayDeaths            int64
	Recovered              int64
	TodayRecovered         int64
	Active                 int64
	Critical               int64
	CasesPerOneMillion     float64
	DeathsPerOneMillion    float64
	Tests                  float64
	TestsPerOneMillion     float64
	Population             int64
	Continent              string
	OneCasePerPeople       int64
	OneDeathPerPeople      int64
	OneTestPerPeople       int64
	ActivePerOneMillion    float64
	RecoveredPerOneMillion float64
	CriticalPerOneMillion  float64
	AffectedCountries      int64
}

type countryInfoStruct struct {
	ID   int64 `json:"_id"`
	Iso2 string
	Iso3 string
	Lat  float64
	Long float64
	Flag string
}
