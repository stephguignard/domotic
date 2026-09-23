# TaHoma — diagnostic et filtrage

## Sonder la box sans jeton

- PIN lisible dans le certificat servi sur **:443** (l'API locale est sur :8443) :
  `openssl s_client -connect <ip>:443 </dev/null 2>/dev/null | openssl x509 -noout -subject`
  → `CN = <pin>.local`. SAN = `<pin>.local`, `gateway-<pin>.local`,
  `gateway-<pin>.dyn.overkiz.com` — d'où la validité de `ServerName: gateway-<pin>.local`.
- Valider la chaîne avant tout code :
  `openssl s_client -connect <ip>:443 -servername gateway-<pin>.local
   -CAfile backend/internal/tahoma/overkiz-root-ca-2048.crt -verify_hostname gateway-<pin>.local`
- **:8443 fermé (connexion refusée) alors que :443 répond = mode développeur non
  activé** (7 appuis sur le PIN dans l'app). Peut aussi se refermer si le NTP
  distribué par le DHCP est invalide.

## Endpoints

- `/setup` ne contient que `devices` et `gateways` : **ni `rootPlace` ni `placeOID`**
  (cloud-only) → aucune pièce disponible, tout tombe dans « Sans pièce ».
- Suivre une commande : `/exec/current` (`label: "domotic"`, `state: IN_PROGRESS`,
  file vidée à la fin). **`/history` n'existe pas en local** (400 `Unknown object`).
- RTS unidirectionnel : `states: []` en permanence, et une commande acceptée ne
  prouve pas que l'équipement a bougé.

## Filtrage de l'infrastructure

`isInfrastructure()` (`internal/tahoma/types.go`) porte sur le **`controllableName`**,
jamais sur le préfixe de `deviceURL` : filtrer `zigbee://` écarterait aussi un vrai
équipement Zigbee appairé plus tard, alors que seul le coordinateur porte un
`Transceiver`.

- Écartés : préfixe `internal:` (PodV3Component, WifiComponent), suffixe `:bridge`
  (ogp:Bridge), contient `transceiver` (zigbee:TransceiverV3_0Component).
- Gardés : `rts:*`, `io:*`, `zigbee:OnOffLightComponent`…
- La migration `0002` ne peut s'appuyer que sur le préfixe (le `controllableName`
  n'est pas stocké) ; un équipement légitime effacé à tort est réinséré au polling
  suivant.
