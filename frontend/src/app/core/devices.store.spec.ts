import { HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { type Confirmation, ConfirmationService, MessageService } from '@openng/optimus-ui/api';

import { DevicesStore } from './devices.store';
import { flushRoomOrder, makeDevice, makeRelay, makeStation, testProviders } from '../testing/providers';

describe('DevicesStore', () => {
  let store: DevicesStore;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: testProviders() });

    store = TestBed.inject(DevicesStore);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  /** Charge un jeu d'équipements dans le store. */
  function load(devices = [makeDevice(), makeStation()]): void {
    store.refresh();
    http.expectOne('/api/devices').flush({ devices, total: devices.length });
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });
    flushRoomOrder();
  }

  it('expose les équipements chargés', () => {
    load();
    expect(store.devices().length).toBe(2);
    expect(store.loading()).toBe(false);
    expect(store.error()).toBeNull();
  });

  it('groupe les équipements par pièce, ordre alphabétique', () => {
    load([
      makeDevice({ id: 'a', room: 'Salon' }),
      makeStation({ id: 'b', room: 'Bureau' }),
      makeDevice({ id: 'c', room: 'Salon' }),
    ]);

    const groups = store.byRoom();
    expect(groups.map((g) => g.room)).toEqual(['Bureau', 'Salon']);
    expect(groups[1].devices.length).toBe(2);
  });

  it('place les équipements sans pièce en dernier', () => {
    // Un équipement Overkiz peut n'être rattaché à aucune pièce ; il ne doit
    // pas pour autant ouvrir la liste.
    load([makeDevice({ id: 'a', room: '' }), makeStation({ id: 'b', room: 'Bureau' })]);

    expect(store.byRoom().map((g) => g.room)).toEqual(['Bureau', 'Sans pièce']);
  });

  it('compte les équipements injoignables', () => {
    load([
      makeDevice({ id: 'a', reachable: false }),
      makeDevice({ id: 'b', reachable: false }),
      makeStation({ id: 'c', reachable: true }),
    ]);

    expect(store.unreachableCount()).toBe(2);
  });

  it('expose les pièces sans doublon ni valeur vide', () => {
    load([
      makeDevice({ id: 'a', room: 'Salon' }),
      makeDevice({ id: 'b', room: 'Salon' }),
      makeStation({ id: 'c', room: '' }),
    ]);

    expect(store.rooms()).toEqual(['Salon']);
  });

  it("retient le message d'erreur du backend", () => {
    store.refresh();

    // Le backend répond en RFC 7807 : le champ `detail` porte le message utile.
    http.expectOne('/api/devices').flush(
      { title: 'Internal Server Error', status: 500, detail: 'lecture des équipements' },
      { status: 500, statusText: 'Internal Server Error' },
    );
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });
    flushRoomOrder();

    expect(store.error()).toBe('lecture des équipements');
    expect(store.loading()).toBe(false);
  });

  it("n'efface pas les équipements quand seule la santé échoue", () => {
    load();
    store.refresh();

    http.expectOne('/api/devices').flush({ devices: [makeDevice()], total: 1 });
    http.expectOne('/api/health').error(new ProgressEvent('error'));
    flushRoomOrder();

    // L'état de santé est secondaire : son échec ne doit pas masquer les
    // équipements, ni être présenté comme une erreur de chargement.
    expect(store.health()).toBeNull();
    expect(store.devices().length).toBe(1);
    expect(store.error()).toBeNull();
  });

  it('envoie une commande et notifie le succès', () => {
    const messages = TestBed.inject(MessageService);
    const added = vi.spyOn(messages, 'add');
    const device = makeDevice();

    load([device]);
    store.sendCommand(device, 'close');

    const req = http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ command: 'close', parameters: [] });
    req.flush({ exec_id: 'exec-1' });

    expect(added).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'success', summary: device.name }),
    );
  });

  it("remonte l'erreur du backend quand une commande échoue", () => {
    const messages = TestBed.inject(MessageService);
    const added = vi.spyOn(messages, 'add');
    const device = makeStation();

    load([device]);
    store.sendCommand(device, 'close');

    http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`).flush(
      { title: 'Unprocessable Entity', status: 422, detail: 'les équipements netatmo ne sont pas pilotables' },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    expect(added).toHaveBeenCalledWith(
      expect.objectContaining({
        severity: 'error',
        detail: 'les équipements netatmo ne sont pas pilotables',
      }),
    );
  });

  describe('commandes confirmées', () => {
    /** Intercepte la demande de confirmation au lieu d'ouvrir le dialogue. */
    function captureConfirmation(): () => Confirmation {
      let captured: Confirmation | undefined;
      vi.spyOn(TestBed.inject(ConfirmationService), 'confirm').mockImplementation(function (
        this: ConfirmationService,
        c: Confirmation,
      ) {
        captured = c;
        return this;
      });
      return () => {
        if (!captured) {
          throw new Error('aucune confirmation demandée');
        }
        return captured;
      };
    }

    it("n'envoie rien avant confirmation sur un relais", () => {
      const confirmation = captureConfirmation();
      const relay = makeRelay();

      load([relay]);
      store.sendCommand(relay, 'off');

      expect(confirmation().header).toBe('Éteindre « Eau chaude » ?');
      http.expectNone(`/api/devices/${encodeURIComponent(relay.id)}/command`);
    });

    it('envoie la commande une fois confirmée', () => {
      const confirmation = captureConfirmation();
      const relay = makeRelay();

      load([relay]);
      store.sendCommand(relay, 'off');
      confirmation().accept?.();

      const req = http.expectOne(`/api/devices/${encodeURIComponent(relay.id)}/command`);
      expect(req.request.body).toEqual({ command: 'off', parameters: [] });
      req.flush({ exec_id: '' });
    });

    it('ne demande pas de confirmation pour un volet', () => {
      const confirm = vi.spyOn(TestBed.inject(ConfirmationService), 'confirm');
      const device = makeDevice();

      load([device]);
      store.sendCommand(device, 'open');

      expect(confirm).not.toHaveBeenCalled();
      http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`).flush({ exec_id: 'x' });
    });
  });

  it("groupe les pièces dans l'ordre choisi par l'utilisateur", () => {
    store.refresh();
    http.expectOne('/api/devices').flush({
      devices: [
        makeDevice({ id: 'a', room: 'Salon' }),
        makeDevice({ id: 'b', room: 'Cuisine' }),
        makeDevice({ id: 'c', room: '' }),
        makeDevice({ id: 'd', room: 'Bureau' }),
      ],
      total: 4,
    });
    flushRoomOrder(['Salon']);
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });

    expect(store.byRoom().map((g) => g.room)).toEqual(['Salon', 'Bureau', 'Cuisine', 'Sans pièce']);
    expect(store.rooms()).toEqual(['Salon', 'Bureau', 'Cuisine']);
  });

  it("enregistre l'ordre des pièces et l'applique aussitôt", () => {
    load([makeDevice({ id: 'a', room: 'Salon' }), makeDevice({ id: 'b', room: 'Cuisine' })]);
    expect(store.rooms()).toEqual(['Cuisine', 'Salon']);

    store.saveRoomOrder(['Salon', 'Cuisine']);
    const req = http.expectOne('/api/rooms/order');
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual({ rooms: ['Salon', 'Cuisine'] });
    req.flush({ rooms: ['Salon', 'Cuisine'] });

    expect(store.rooms()).toEqual(['Salon', 'Cuisine']);
  });
});
