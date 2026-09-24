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

import { Device, provideApi } from '../api';
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

/**
 * Répond aux deux appels que `DevicesStore.refresh()` déclenche.
 *
 * À appeler après `TestBed.createComponent`, sinon `HttpTestingController.verify()`
 * échouera sur des requêtes en attente.
 */
export function flushRefresh(devices: Device[] = []): void {
  const http = TestBed.inject(HttpTestingController);

  http.expectOne('/api/devices').flush({ devices, total: devices.length });
  http.expectOne('/api/health').flush({
    status: 'ok',
    database: true,
    sources: {
      netatmo: { enabled: true, healthy: true },
      tahoma: { enabled: true, healthy: true },
    },
  });
}
