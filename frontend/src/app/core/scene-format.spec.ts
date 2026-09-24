import { SceneStep } from '../api';
import {
  describeDays,
  describeSchedule,
  describeStep,
  formatDuration,
  sceneStatus,
} from './scene-format';
import { makeScene } from '../testing/providers';

describe('describeDays', () => {
  it('nomme les combinaisons courantes', () => {
    expect(describeDays([1, 2, 3, 4, 5, 6, 7])).toBe('Tous les jours');
    expect(describeDays([5, 4, 3, 2, 1])).toBe('En semaine');
    expect(describeDays([6, 7])).toBe('Le week-end');
    expect(describeDays([1, 3, 5])).toBe('Lun, Mer, Ven');
  });
});

describe('describeSchedule', () => {
  it('décrit une heure fixe', () => {
    expect(
      describeSchedule({ enabled: true, days: [1, 2, 3, 4, 5], at: 'time', time: '07:30' }),
    ).toBe('En semaine · 07:30');
  });

  it('décrit un horaire solaire avec décalage et bornes', () => {
    expect(
      describeSchedule({
        enabled: true,
        days: [6, 7],
        at: 'sunset',
        offset_minutes: -15,
        not_before: '18:00',
      }),
    ).toBe('Le week-end · coucher du soleil −15 min (pas avant 18:00)');
    expect(describeSchedule({ enabled: true, days: [1], at: 'sunrise', offset_minutes: 30 })).toBe(
      'Lun · lever du soleil +30 min',
    );
  });
});

describe('describeStep', () => {
  const name = (id: string) => ({ 'l-1': 'Plafonnier' })[id] ?? id;

  it('décrit une action, sa valeur, ses cibles et son filtre', () => {
    const step: SceneStep = {
      type: 'action',
      command: 'setBrightness',
      parameters: [40],
      targets: { devices: ['l-1'], rooms: ['Salon'], kinds: ['light'] },
    };
    expect(describeStep(step, name)).toBe('Luminosité 40 % · Salon, Plafonnier (lumières)');
  });

  it('décrit une attente', () => {
    expect(describeStep({ type: 'wait', wait_minutes: 90 }, name)).toBe('Attendre 1 h 30');
    expect(formatDuration(5)).toBe('5 min');
    expect(formatDuration(120)).toBe('2 h');
  });
});

describe('sceneStatus', () => {
  it("montre une exécution en cours avant l'issue précédente", () => {
    expect(sceneStatus(makeScene({ running: true, last_status: 'failed' }))?.label).toBe(
      'En cours',
    );
    expect(sceneStatus(makeScene({ last_status: 'partial' }))?.severity).toBe('warn');
    expect(sceneStatus(makeScene())).toBeNull();
  });
});
