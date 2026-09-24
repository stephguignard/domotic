/**
 * Mise en forme des scènes : libellés des actions, des jours, des horaires.
 *
 * Fonctions pures, comme `device-state.ts` : elles se testent sans rendu, et
 * la page Scènes comme le tableau de bord les partagent.
 */
import { SceneSchedule, SceneStep, SceneView } from '../api';
import { TagSeverity } from './device-state';

/** Actions proposées dans une scène, dans l'ordre du menu. */
export const SCENE_ACTIONS: { value: SceneStep.CommandEnum; label: string }[] = [
  { value: 'on', label: 'Allumer' },
  { value: 'off', label: 'Éteindre' },
  { value: 'open', label: 'Ouvrir' },
  { value: 'close', label: 'Fermer' },
  { value: 'stop', label: 'Arrêter' },
  { value: 'setBrightness', label: 'Luminosité' },
  { value: 'setColorTemperature', label: 'Blanc' },
  { value: 'setColor', label: 'Couleur' },
];

/** Types proposés pour restreindre une action sur une pièce. */
export const SCENE_KINDS: { value: string; label: string }[] = [
  { value: 'light', label: 'Lumières' },
  { value: 'switch', label: 'Relais' },
  { value: 'shutter', label: 'Volets' },
  { value: 'awning', label: 'Stores' },
  { value: 'window', label: 'Fenêtres' },
  { value: 'gate', label: 'Portails' },
];

/** Initiales et noms des jours, du lundi (1) au dimanche (7). */
export const DAYS: { value: number; short: string; long: string }[] = [
  { value: 1, short: 'L', long: 'Lun' },
  { value: 2, short: 'M', long: 'Mar' },
  { value: 3, short: 'M', long: 'Mer' },
  { value: 4, short: 'J', long: 'Jeu' },
  { value: 5, short: 'V', long: 'Ven' },
  { value: 6, short: 'S', long: 'Sam' },
  { value: 7, short: 'D', long: 'Dim' },
];

/** Paramètre par défaut d'une action, quand elle en prend un. */
export function defaultParameters(command: SceneStep.CommandEnum): unknown[] {
  switch (command) {
    case 'setBrightness':
      return [50];
    case 'setColorTemperature':
      return [2700];
    case 'setColor':
      return ['#ffb35c'];
    default:
      return [];
  }
}

/** Décrit des jours : « Tous les jours », « En semaine », « Lun, Mer, Ven »… */
export function describeDays(days: number[]): string {
  const set = new Set(days);
  const is = (...wanted: number[]) => set.size === wanted.length && wanted.every((d) => set.has(d));
  if (is(1, 2, 3, 4, 5, 6, 7)) return 'Tous les jours';
  if (is(1, 2, 3, 4, 5)) return 'En semaine';
  if (is(6, 7)) return 'Le week-end';
  return DAYS.filter((d) => set.has(d.value))
    .map((d) => d.long)
    .join(', ');
}

/** Décrit un horaire : « En semaine · 07:30 », « Tous les jours · coucher du soleil −15 min ». */
export function describeSchedule(s: SceneSchedule): string {
  const days = describeDays(s.days);
  if (s.at === 'time') {
    return `${days} · ${s.time ?? '—'}`;
  }

  let when = s.at === 'sunrise' ? 'lever du soleil' : 'coucher du soleil';
  const offset = s.offset_minutes ?? 0;
  if (offset !== 0) {
    // Signe typographique « − » plutôt que le trait d'union.
    when += ` ${offset > 0 ? '+' : '−'}${Math.abs(offset)} min`;
  }
  const bounds = [
    s.not_before ? `pas avant ${s.not_before}` : '',
    s.not_after ? `pas après ${s.not_after}` : '',
  ].filter(Boolean);
  return `${days} · ${when}${bounds.length ? ` (${bounds.join(', ')})` : ''}`;
}

/** Décrit l'action d'une étape, valeur comprise : « Luminosité 40 % ». */
export function describeAction(step: SceneStep): string {
  const label = SCENE_ACTIONS.find((a) => a.value === step.command)?.label ?? step.command ?? '';
  const [value] = step.parameters ?? [];
  switch (step.command) {
    case 'setBrightness':
      return `${label} ${value} %`;
    case 'setColorTemperature':
      return `${label} ${value} K`;
    case 'setColor':
      return `${label} ${value}`;
    default:
      return label;
  }
}

/**
 * Décrit une étape : « Fermer · Salon, Cuisine (volets) », « Attendre 5 min ».
 * `deviceName` traduit un identifiant d'équipement en nom lisible.
 */
export function describeStep(step: SceneStep, deviceName: (id: string) => string): string {
  if (step.type === 'wait') {
    return `Attendre ${formatDuration(step.wait_minutes ?? 0)}`;
  }
  const t = step.targets;
  const targets = [...(t?.rooms ?? []), ...(t?.devices ?? []).map(deviceName)].join(', ');
  const kinds = (t?.kinds ?? [])
    .map((k) => SCENE_KINDS.find((o) => o.value === k)?.label.toLowerCase() ?? k)
    .join(', ');
  return `${describeAction(step)} · ${targets || 'aucune cible'}${kinds ? ` (${kinds})` : ''}`;
}

/** Durée lisible : « 5 min », « 1 h 30 ». */
export function formatDuration(minutes: number): string {
  if (minutes < 60) return `${minutes} min`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m ? `${h} h ${String(m).padStart(2, '0')}` : `${h} h`;
}

/** Libellé et couleur de l'issue de la dernière exécution. */
export function sceneStatus(
  scene: SceneView,
): { label: string; severity: TagSeverity | 'danger' } | null {
  if (scene.running) {
    return { label: 'En cours', severity: 'info' };
  }
  switch (scene.last_status) {
    case 'success':
      return { label: 'Réussie', severity: 'success' };
    case 'partial':
      return { label: 'Partiellement réussie', severity: 'warn' };
    case 'failed':
      return { label: 'Échec', severity: 'danger' };
    case 'interrupted':
      return { label: 'Interrompue', severity: 'secondary' };
    default:
      return null;
  }
}
