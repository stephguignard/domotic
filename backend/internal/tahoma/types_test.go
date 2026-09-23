package tahoma

import "testing"

func TestIsInfrastructure(t *testing.T) {
	cases := []struct {
		controllableName string
		want             bool
	}{
		// Relevés sur une box réelle : la passerelle expose ses propres
		// composants et ses ponts de protocole au même titre que les
		// équipements.
		{"internal:PodV3Component", true},
		{"internal:WifiComponent", true},
		{"ogp:Bridge", true},
		{"zigbee:TransceiverV3_0Component", true},

		// Vrais équipements de la même installation.
		{"rts:ExteriorBlindRTSComponent", false},
		{"rts:RollerShutterRTSComponent", false},

		// Un équipement Zigbee appairé plus tard ne doit pas être écarté :
		// c'est la raison pour laquelle le filtrage ne porte pas sur le
		// préfixe de protocole.
		{"zigbee:OnOffLightComponent", false},
		{"io:RollerShutterGenericIOComponent", false},
		{"", false},
	}

	for _, c := range cases {
		if got := isInfrastructure(c.controllableName); got != c.want {
			t.Errorf("isInfrastructure(%q) = %v, attendu %v", c.controllableName, got, c.want)
		}
	}
}

func TestKindForControllable(t *testing.T) {
	cases := map[string]string{
		"rts:ExteriorBlindRTSComponent":       "shutter",
		"rts:RollerShutterRTSComponent":       "shutter",
		"io:RollerShutterGenericIOComponent":  "shutter",
		"io:AwningValanceIOComponent":         "awning",
		"io:GarageOpenerIOComponent":          "gate",
		"zigbee:OnOffLightComponent":          "light",
		"io:SomfyThermostatTemperatureSensor": "sensor",
		"inconnu:Chose":                       "unknown",
	}

	for name, want := range cases {
		if got := kindForControllable(name); got != want {
			t.Errorf("kindForControllable(%q) = %q, attendu %q", name, got, want)
		}
	}
}
