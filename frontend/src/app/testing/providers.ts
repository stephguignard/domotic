/**
 * Outillage commun aux tests de composants.
 *
 * Les composants consomment tous `DevicesStore`, qui dépend lui-même du client
 * API généré et de `MessageService`. Plutôt que de simuler le store — ce qui
 * reviendrait à tester des doublures — les specs utilisent le vrai store et
 * interceptent les requêtes HTTP : ce qui est vérifié est alors le
 * comportement réel, du décodage de la réponse jusqu'au rendu.
 */
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { EnvironmentProviders, Provider } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { ConfirmationService, MessageService } from '@openng/optimus-ui/api';

import { Device, SceneView, provideApi } from '../api';
import { routes } from '../app.routes';

/** Providers nécessaires à tout composant de l'application. */
export function testProviders(): (Provider | EnvironmentProviders)[] {
  return [
    provideRouter(routes),
    provideHttpClient(),
    provideHttpClientTesting(),
    provideApi(''),
    MessageService,
    ConfirmationService,
  ];
}

/** Construit un équipement de test, surchargeable champ par champ. */
export function makeDevice(overrides: Partial<Device> = {}): Device {
  return {
    id: 'io://1234-5678-9012/1',
    source: 'tahoma',
    name: 'Volet salon',
    kind: 'shutter',
    room: 'Salon',
    source_room: 'Salon',
    room_overridden: false,
    state: '{"core:ClosureState":100}',
    reachable: true,
    updated_at: '2026-09-23T18:00:00Z',
    ...overrides,
  };
}

/** Construit une station météo Netatmo de test. */
export function makeStation(overrides: Partial<Device> = {}): Device {
  return makeDevice({
    id: '70:ee:50:00:00:01',
    source: 'netatmo',
    name: 'Station',
    kind: 'weather_station',
    room: 'Bureau',
    state: '{"temperature":21.5,"humidity":48,"co2":512,"noise":35,"pressure":1013}',
    ...overrides,
  });
}

/** Construit une voie de module Shelly de test. */
export function makeRelay(overrides: Partial<Device> = {}): Device {
  return makeDevice({
    id: 'shellypro3-841fe88e5b68:switch:0',
    source: 'shelly',
    name: 'Eau chaude',
    kind: 'switch',
    room: '',
    state: '{"on":true,"device_temperature":44}',
    ...overrides,
  });
}

/** Construit une lampe couleur Hue de test, réglée sur un blanc chaud. */
export function makeLight(overrides: Partial<Device> = {}): Device {
  return makeDevice({
    id: '8c2d7a3e-0000-4000-8000-000000000001',
    source: 'hue',
    name: 'Plafonnier',
    kind: 'light',
    room: 'Salon',
    state: '{"on":true,"brightness":56.92,"color":"#ffb35c","color_temperature":2240}',
    ...overrides,
  });
}

/**
 * Répond aux deux appels que `DevicesStore.refresh()` déclenche.
 *
 * À appeler après `TestBed.createComponent`, sinon `HttpTestingController.verify()`
 * échouera sur des requêtes en attente.
 */
export function flushRefresh(devices: Device[] = [], roomOrder: string[] = []): void {
  const http = TestBed.inject(HttpTestingController);

  http.expectOne('/api/devices').flush({ devices, total: devices.length });
  flushRoomOrder(roomOrder);
  http.expectOne('/api/health').flush({
    status: 'ok',
    database: true,
    sources: {
      netatmo: { enabled: true, healthy: true },
      tahoma: { enabled: true, healthy: true },
    },
  });
}

/** Construit une scène de test, surchargeable champ par champ. */
export function makeScene(overrides: Partial<SceneView> = {}): SceneView {
  return {
    id: 1,
    name: 'Soirée',
    show_on_dashboard: true,
    steps: [
      {
        type: 'action',
        command: 'on',
        parameters: [],
        targets: { devices: [], rooms: ['Salon'], kinds: [] },
      },
    ],
    schedules: [],
    running: false,
    touches_relays: false,
    created_at: '2026-09-24T08:00:00Z',
    updated_at: '2026-09-24T08:00:00Z',
    ...overrides,
  };
}

/** Répond à la lecture des scènes, secondaire pour la plupart des cas. */
export function flushScenes(scenes: SceneView[] = []): void {
  TestBed.inject(HttpTestingController)
    .match('/api/scenes')
    .forEach((req) => req.flush({ scenes, solar_available: true, time_zone: 'Europe/Zurich' }));
}

/** Répond à la lecture de l'ordre des pièces que `DevicesStore.refresh()` déclenche. */
export function flushRoomOrder(rooms: string[] = []): void {
  TestBed.inject(HttpTestingController).expectOne('/api/rooms/order').flush({ rooms });
}
