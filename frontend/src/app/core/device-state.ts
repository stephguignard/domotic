/**
 * Utilitaires de présentation des équipements.
 *
 * Le backend expose l'état d'un équipement en JSON sérialisé dans une chaîne :
 * la forme varie trop d'une source et d'un type à l'autre pour être typée. Ces
 * fonctions le ramènent à ce que l'interface sait afficher.
 */
import { Device } from '../api';

/** État décodé d'un équipement : des grandeurs nommées, de types variés. */
export type DeviceState = Record<string, unknown>;

/** Décode l'état d'un équipement, en tolérant une chaîne absente ou invalide. */
export function parseState(device: Device): DeviceState {
  if (!device.state) {
    return {};
  }
  try {
    const parsed: unknown = JSON.parse(device.state);
    // Un tableau passe le test `typeof === 'object'` mais ne décrit pas un
    // état : le laisser passer produirait des libellés « 0 », « 1 »…
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {};
    }
    return parsed as DeviceState;
  } catch {
    return {};
  }
}

/** Libellés français des grandeurs les plus courantes. */
const METRIC_LABELS: Record<string, string> = {
  temperature: 'Température',
  humidity: 'Humidité',
  co2: 'CO₂',
  noise: 'Bruit',
  pressure: 'Pression',
  absolute_pressure: 'Pression absolue',
  rain: 'Pluie',
  rain_1h: 'Pluie (1 h)',
  rain_24h: 'Pluie (24 h)',
  wind_strength: 'Vent',
  wind_angle: 'Direction du vent',
  gust_strength: 'Rafales',
  gust_angle: 'Direction des rafales',
  battery_percent: 'Batterie',
  'core:ClosureState': 'Fermeture',
  'core:OpenClosedState': 'Ouverture',
  'core:StatusState': 'Statut',
  'core:RSSILevelState': 'Signal',
  'core:BatteryState': 'Batterie',
  'core:TargetClosureState': 'Fermeture visée',
  on: 'Allumé',
  brightness: 'Luminosité',
  device_temperature: 'Température interne',
};

/** Unités associées aux grandeurs, quand elles en ont une. */
const METRIC_UNITS: Record<string, string> = {
  temperature: '°C',
  humidity: '%',
  co2: 'ppm',
  noise: 'dB',
  pressure: 'hPa',
  absolute_pressure: 'hPa',
  rain: 'mm',
  rain_1h: 'mm',
  rain_24h: 'mm',
  wind_strength: 'km/h',
  wind_angle: '°',
  gust_strength: 'km/h',
  gust_angle: '°',
  battery_percent: '%',
  'core:ClosureState': '%',
  'core:TargetClosureState': '%',
  'core:RSSILevelState': '%',
  brightness: '%',
  device_temperature: '°C',
};

/** Retourne le libellé lisible d'une grandeur. */
export function metricLabel(metric: string): string {
  return METRIC_LABELS[metric] ?? metric.replace(/^core:/, '').replace(/State$/, '');
}

/** Retourne l'unité d'une grandeur, ou une chaîne vide. */
export function metricUnit(metric: string): string {
  return METRIC_UNITS[metric] ?? '';
}

/** Met en forme une valeur d'état pour l'affichage, unité comprise. */
export function formatValue(metric: string, value: unknown): string {
  if (value === null || value === undefined) {
    return '—';
  }
  if (typeof value === 'boolean') {
    return value ? 'Oui' : 'Non';
  }

  const unit = metricUnit(metric);
  if (typeof value === 'number') {
    // Une décimale suffit pour des mesures domestiques ; les entiers restent entiers.
    const formatted = Number.isInteger(value) ? String(value) : value.toFixed(1);
    return unit ? `${formatted} ${unit}` : formatted;
  }
  return unit ? `${String(value)} ${unit}` : String(value);
}

/** Libellés français des types d'équipement. */
const KIND_LABELS: Record<string, string> = {
  weather_station: 'Station météo',
  outdoor_module: 'Module extérieur',
  indoor_module: 'Module intérieur',
  wind_gauge: 'Anémomètre',
  rain_gauge: 'Pluviomètre',
  camera: 'Caméra',
  shutter: 'Volet',
  awning: 'Store',
  window: 'Fenêtre',
  gate: 'Portail',
  light: 'Éclairage',
  switch: 'Relais',
  sensor: 'Capteur',
  alarm: 'Alarme',
  thermostat: 'Thermostat',
  gateway: 'Passerelle',
  unknown: 'Inconnu',
};

/** Retourne le libellé lisible d'un type d'équipement. */
export function kindLabel(kind: string): string {
  return KIND_LABELS[kind] ?? kind;
}

/** Icône Optimus associée à un type d'équipement. */
export function kindIcon(kind: string): string {
  switch (kind) {
    case 'weather_station':
    case 'outdoor_module':
    case 'indoor_module':
      return 'pi pi-cloud';
    case 'wind_gauge':
      return 'pi pi-compass';
    case 'rain_gauge':
      return 'pi pi-cloud-download';
    case 'camera':
      return 'pi pi-video';
    case 'shutter':
    case 'awning':
    case 'window':
      return 'pi pi-window-minimize';
    case 'gate':
      return 'pi pi-car';
    case 'light':
      return 'pi pi-lightbulb';
    case 'switch':
      return 'pi pi-power-off';
    case 'alarm':
      return 'pi pi-shield';
    case 'thermostat':
      return 'pi pi-sun';
    case 'gateway':
      return 'pi pi-server';
    default:
      return 'pi pi-box';
  }
}

/** Sources dont les équipements se pilotent. L'API météo Netatmo est en
 * lecture seule, et le backend rejette toute commande qui lui serait adressée. */
const CONTROLLABLE_SOURCES = new Set(['tahoma', 'hue', 'shelly']);

/** Indique si un équipement accepte des commandes. */
export function isControllable(device: Device): boolean {
  return CONTROLLABLE_SOURCES.has(device.source) && device.reachable;
}

/**
 * Indique si une commande doit être confirmée avant envoi.
 *
 * Un relais commande une charge dont on ne voit pas l'effet depuis
 * l'interface — chauffe-eau, chauffage — et qu'un clic égaré couperait sans
 * que personne ne s'en aperçoive avant d'avoir froid.
 */
export function needsConfirmation(device: Device): boolean {
  return device.kind === 'switch';
}

/** Sévérité d'un tag Optimus, telle qu'acceptée par `<p-tag>`. */
export type TagSeverity = 'info' | 'success' | 'warn' | 'secondary' | 'contrast';

/** Présentation de chaque source : libellé et couleur de son tag. */
const SOURCES: Record<string, { label: string; severity: TagSeverity }> = {
  netatmo: { label: 'Netatmo', severity: 'info' },
  tahoma: { label: 'Somfy TaHoma', severity: 'success' },
  hue: { label: 'Philips Hue', severity: 'warn' },
  shelly: { label: 'Shelly', severity: 'contrast' },
};

/** Retourne le libellé lisible d'une source. */
export function sourceLabel(source: string): string {
  return SOURCES[source]?.label ?? source;
}

/** Retourne la sévérité du tag d'une source. */
export function sourceSeverity(source: string): TagSeverity {
  return SOURCES[source]?.severity ?? 'secondary';
}

/** Sources connues, pour les filtres. */
export const SOURCE_IDS = Object.keys(SOURCES);

/**
 * Commandes proposées pour un type d'équipement.
 *
 * Cette liste est volontairement conservatrice : l'API locale expose la liste
 * exacte des commandes de chaque équipement, mais ces quelques-unes couvrent
 * les usages courants et sont acceptées par tous les équipements de ce type.
 */
export function commandsFor(kind: string): { label: string; command: string; icon: string }[] {
  switch (kind) {
    case 'shutter':
    case 'awning':
    case 'window':
      return [
        { label: 'Ouvrir', command: 'open', icon: 'pi pi-angle-up' },
        { label: 'Stop', command: 'stop', icon: 'pi pi-pause' },
        { label: 'Fermer', command: 'close', icon: 'pi pi-angle-down' },
      ];
    case 'gate':
      return [
        { label: 'Ouvrir', command: 'open', icon: 'pi pi-angle-up' },
        { label: 'Fermer', command: 'close', icon: 'pi pi-angle-down' },
      ];
    case 'light':
      return [
        { label: 'Allumer', command: 'on', icon: 'pi pi-lightbulb' },
        { label: 'Éteindre', command: 'off', icon: 'pi pi-power-off' },
      ];
    case 'switch':
      return [
        { label: 'Allumer', command: 'on', icon: 'pi pi-play' },
        { label: 'Éteindre', command: 'off', icon: 'pi pi-power-off' },
      ];
    default:
      return [];
  }
}
