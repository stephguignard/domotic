package netatmo

// Types de modules Netatmo, tels que renvoyés par le champ `type`.
const (
	TypeMain    = "NAMain"    // station intérieure principale
	TypeOutdoor = "NAModule1" // module extérieur : température, humidité
	TypeWind    = "NAModule2" // anémomètre
	TypeRain    = "NAModule3" // pluviomètre
	TypeIndoor  = "NAModule4" // module intérieur additionnel
)

// stationsDataResponse correspond à la réponse de /api/getstationsdata.
type stationsDataResponse struct {
	Body struct {
		Devices []station `json:"devices"`
	} `json:"body"`
	Status string `json:"status"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// station est une station météo principale et ses modules rattachés.
type station struct {
	ID          string    `json:"_id"`
	StationName string    `json:"station_name"`
	ModuleName  string    `json:"module_name"`
	Type        string    `json:"type"`
	Reachable   bool      `json:"reachable"`
	DataType    []string  `json:"data_type"`
	Dashboard   dashboard `json:"dashboard_data"`
	Place       struct {
		City     string  `json:"city"`
		Country  string  `json:"country"`
		Altitude float64 `json:"altitude"`
	} `json:"place"`
	Modules []module `json:"modules"`
}

// module est un module rattaché à une station.
type module struct {
	ID         string    `json:"_id"`
	ModuleName string    `json:"module_name"`
	Type       string    `json:"type"`
	Reachable  bool      `json:"reachable"`
	DataType   []string  `json:"data_type"`
	BatteryPct int       `json:"battery_percent"`
	Dashboard  dashboard `json:"dashboard_data"`
}

// dashboard porte les mesures courantes d'un module.
//
// Les champs sont des pointeurs : Netatmo omet ceux que le module ne mesure
// pas, et une température de 0 °C est une valeur parfaitement légitime qu'il
// ne faut pas confondre avec une absence de mesure.
type dashboard struct {
	TimeUTC      int64    `json:"time_utc"`
	Temperature  *float64 `json:"Temperature"`
	Humidity     *float64 `json:"Humidity"`
	CO2          *float64 `json:"CO2"`
	Noise        *float64 `json:"Noise"`
	Pressure     *float64 `json:"Pressure"`
	AbsPressure  *float64 `json:"AbsolutePressure"`
	Rain         *float64 `json:"Rain"`
	SumRain1     *float64 `json:"sum_rain_1"`
	SumRain24    *float64 `json:"sum_rain_24"`
	WindStrength *float64 `json:"WindStrength"`
	WindAngle    *float64 `json:"WindAngle"`
	GustStrength *float64 `json:"GustStrength"`
	GustAngle    *float64 `json:"GustAngle"`
	MinTemp      *float64 `json:"min_temp"`
	MaxTemp      *float64 `json:"max_temp"`
}

// metrics aplatit le tableau de bord en couples (grandeur, valeur), en
// ignorant les mesures absentes.
func (d dashboard) metrics() map[string]float64 {
	out := make(map[string]float64, 8)
	for name, v := range map[string]*float64{
		"temperature":       d.Temperature,
		"humidity":          d.Humidity,
		"co2":               d.CO2,
		"noise":             d.Noise,
		"pressure":          d.Pressure,
		"absolute_pressure": d.AbsPressure,
		"rain":              d.Rain,
		"rain_1h":           d.SumRain1,
		"rain_24h":          d.SumRain24,
		"wind_strength":     d.WindStrength,
		"wind_angle":        d.WindAngle,
		"gust_strength":     d.GustStrength,
		"gust_angle":        d.GustAngle,
	} {
		if v != nil {
			out[name] = *v
		}
	}
	return out
}

// homeDataResponse correspond à la réponse de /api/gethomedata (sécurité).
type homeDataResponse struct {
	Body struct {
		Homes []home `json:"homes"`
	} `json:"body"`
	Status string `json:"status"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type home struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Cameras []camera `json:"cameras"`
}

type camera struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Status     string `json:"status"` // "on" | "off"
	SDStatus   string `json:"sd_status"`
	AlimStatus string `json:"alim_status"`
	IsLocal    bool   `json:"is_local"`
}

// kindForType traduit un type de module Netatmo en type d'équipement unifié.
func kindForType(t string) string {
	switch t {
	case TypeMain:
		return "weather_station"
	case TypeOutdoor:
		return "outdoor_module"
	case TypeWind:
		return "wind_gauge"
	case TypeRain:
		return "rain_gauge"
	case TypeIndoor:
		return "indoor_module"
	default:
		return "sensor"
	}
}
