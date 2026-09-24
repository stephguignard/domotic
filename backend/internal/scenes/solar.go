package scenes

import (
	"math"
	"time"
)

// sunTimes calcule le lever et le coucher du soleil pour une date civile, en
// un lieu donné (degrés décimaux, longitude positive à l'est).
//
// Méthode de l'équation du lever du soleil (forme simplifiée), qui tient
// compte de la réfraction et du diamètre apparent du soleil (−0,833°).
// Vérifiée contre l'US Naval Observatory : à la minute près sous nos
// latitudes, largement assez pour ouvrir des volets.
//
// ok est faux quand le soleil ne se lève ou ne se couche pas ce jour-là
// (au-delà des cercles polaires).
func sunTimes(year int, month time.Month, day int, lat, lon float64) (rise, set time.Time, ok bool) {
	// Jours écoulés depuis J2000 (1er janvier 2000, 12:00 TT), au midi UTC
	// de la date, corrigés de la longitude.
	noon := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	n := float64(noon.Unix())/86400 + 2440587.5 - 2451545.0
	jStar := n + 0.0009 - lon/360

	m := normalize(357.5291 + 0.98560028*jStar) // anomalie moyenne
	mr := rad(m)
	c := 1.9148*math.Sin(mr) + 0.0200*math.Sin(2*mr) + 0.0003*math.Sin(3*mr) // équation du centre
	lambda := rad(normalize(m + c + 180 + 102.9372))                         // longitude écliptique

	jTransit := 2451545.0 + jStar + 0.0053*math.Sin(mr) - 0.0069*math.Sin(2*lambda)
	decl := math.Asin(math.Sin(lambda) * math.Sin(rad(23.4397)))

	cosH := (math.Sin(rad(-0.833)) - math.Sin(rad(lat))*math.Sin(decl)) / (math.Cos(rad(lat)) * math.Cos(decl))
	if cosH < -1 || cosH > 1 {
		return time.Time{}, time.Time{}, false
	}
	h := deg(math.Acos(cosH)) / 360

	return julianToTime(jTransit - h), julianToTime(jTransit + h), true
}

// deltaT est l'avance du temps terrestre, dans lequel la méthode exprime ses
// dates juliennes, sur le temps universel des horloges : environ 69 s dans les
// années 2020. Sans cette correction, tous les horaires arrivent une minute
// trop tard.
const deltaT = 69 * time.Second

func julianToTime(j float64) time.Time {
	secs := (j - 2440587.5) * 86400
	return time.Unix(int64(math.Round(secs)), 0).UTC().Add(-deltaT)
}

func normalize(d float64) float64 {
	d = math.Mod(d, 360)
	if d < 0 {
		d += 360
	}
	return d
}

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }
