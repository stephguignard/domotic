import { Device } from '../api';
import {
  commandsFor,
  describeCommand,
  formatValue,
  isControllable,
  lightSettings,
  metricLabel,
  needsConfirmation,
  parseState,
  sourceLabel,
  sourceSeverity,
} from './device-state';

function device(overrides: Partial<Device> = {}): Device {
  return {
    id: 'io://1234-5678-9012/1',
    source: 'tahoma',
    name: 'Volet salon',
    kind: 'shutter',
    room: 'Salon',
    state: '{}',
    reachable: true,
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('parseState', () => {
  it("décode l'état JSON", () => {
    const state = parseState(device({ state: '{"core:ClosureState":100}' }));
    expect(state['core:ClosureState']).toBe(100);
  });

  it('tolère un état vide ou invalide', () => {
    // L'état vient d'une source externe : une chaîne illisible ne doit pas
    // faire tomber l'affichage de tout un équipement.
    expect(parseState(device({ state: '' }))).toEqual({});
    expect(parseState(device({ state: 'pas du json' }))).toEqual({});
    expect(parseState(device({ state: 'null' }))).toEqual({});
    // Un tableau est du JSON valide mais ne décrit pas un état.
    expect(parseState(device({ state: '[1,2]' }))).toEqual({});
  });
});

describe('formatValue', () => {
  it("ajoute l'unité de la grandeur", () => {
    expect(formatValue('temperature', 21.53)).toBe('21.5 °C');
    expect(formatValue('humidity', 48)).toBe('48 %');
    expect(formatValue('co2', 512)).toBe('512 ppm');
  });

  it('laisse les grandeurs sans unité telles quelles', () => {
    expect(formatValue('core:StatusState', 'available')).toBe('available');
  });

  it('traduit les booléens', () => {
    expect(formatValue('core:OnOffState', true)).toBe('Oui');
    expect(formatValue('core:OnOffState', false)).toBe('Non');
  });

  it('gère une valeur absente', () => {
    expect(formatValue('temperature', null)).toBe('—');
    expect(formatValue('temperature', undefined)).toBe('—');
  });

  it('préserve une mesure nulle', () => {
    // 0 °C est une température parfaitement légitime : la confondre avec une
    // absence de mesure serait une régression visible en hiver.
    expect(formatValue('temperature', 0)).toBe('0 °C');
  });
});

describe('metricLabel', () => {
  it('traduit les grandeurs connues', () => {
    expect(metricLabel('temperature')).toBe('Température');
    expect(metricLabel('core:ClosureState')).toBe('Fermeture');
  });

  it("nettoie les états Overkiz qu'il ne connaît pas", () => {
    expect(metricLabel('core:SomethingState')).toBe('Something');
  });
});

describe('isControllable', () => {
  it('accepte un équipement TaHoma joignable', () => {
    expect(isControllable(device())).toBe(true);
  });

  it('refuse un équipement injoignable', () => {
    expect(isControllable(device({ reachable: false }))).toBe(false);
  });

  it('accepte les équipements Hue et Shelly joignables', () => {
    expect(isControllable(device({ source: 'hue', kind: 'light' }))).toBe(true);
    expect(isControllable(device({ source: 'shelly', kind: 'switch' }))).toBe(true);
  });

  it('refuse les équipements Netatmo', () => {
    // L'API météo Netatmo est en lecture seule ; le backend rejette de toute
    // façon la commande, autant ne pas proposer le bouton.
    expect(isControllable(device({ source: 'netatmo', kind: 'weather_station' }))).toBe(false);
  });
});

describe('sourceLabel / sourceSeverity', () => {
  it('nomme et colore chaque source connue différemment', () => {
    const ids = ['netatmo', 'tahoma', 'hue', 'shelly'];
    expect(ids.map(sourceLabel)).toEqual(['Netatmo', 'Somfy TaHoma', 'Philips Hue', 'Shelly']);
    expect(new Set(ids.map(sourceSeverity)).size).toBe(ids.length);
  });

  it("retombe sur l'identifiant brut pour une source inconnue", () => {
    expect(sourceLabel('zwave')).toBe('zwave');
    expect(sourceSeverity('zwave')).toBe('secondary');
  });
});

describe('commandsFor', () => {
  it('propose ouverture, stop et fermeture pour un volet', () => {
    expect(commandsFor('shutter').map((c) => c.command)).toEqual(['open', 'stop', 'close']);
  });

  it('propose marche et arrêt pour un relais', () => {
    expect(commandsFor('switch').map((c) => c.command)).toEqual(['on', 'off']);
  });

  it('ne propose rien pour un capteur', () => {
    expect(commandsFor('weather_station')).toEqual([]);
  });
});

describe('needsConfirmation', () => {
  it('exige une confirmation pour un relais, pas pour une lumière ni un volet', () => {
    expect(needsConfirmation(device({ source: 'shelly', kind: 'switch' }))).toBe(true);
    expect(needsConfirmation(device({ source: 'hue', kind: 'light' }))).toBe(false);
    expect(needsConfirmation(device())).toBe(false);
  });
});

describe('lightSettings', () => {
  const light = (state: string) => device({ source: 'hue', kind: 'light', state });

  it('expose les réglages que la lampe sait faire', () => {
    expect(
      lightSettings(
        light('{"on":true,"brightness":57,"color":"#ffb35c","color_temperature":2240}'),
      ),
    ).toEqual({
      brightness: 57,
      color: '#ffb35c',
      colorTemperature: 2240,
    });
  });

  it("distingue une lampe réglée sur une couleur d'une lampe sans blancs", () => {
    expect(
      lightSettings(light('{"color":"#ff0000","color_temperature":null}')).colorTemperature,
    ).toBeNull();
    expect('colorTemperature' in lightSettings(light('{"color":"#ff0000"}'))).toBe(false);
  });

  it('ne propose rien pour une prise ou un autre type', () => {
    expect(lightSettings(light('{"on":false}'))).toEqual({});
    expect(lightSettings(device({ kind: 'shutter', state: '{"brightness":50}' }))).toEqual({});
  });

  it('écarte une couleur mal formée', () => {
    expect(lightSettings(light('{"color":"rouge"}')).color).toBeUndefined();
  });
});

describe('describeCommand', () => {
  it('décrit les réglages avec leur valeur', () => {
    expect(describeCommand('light', 'setBrightness', [40])).toBe('Luminosité réglée à 40 %');
    expect(describeCommand('light', 'setColorTemperature', [2700])).toBe('Blanc réglé à 2700 K');
  });

  it('reprend le libellé du bouton pour les commandes simples', () => {
    expect(describeCommand('shutter', 'close')).toBe('Commande « Fermer » envoyée');
    expect(describeCommand('shutter', 'inconnue')).toBe('Commande « inconnue » envoyée');
  });
});
