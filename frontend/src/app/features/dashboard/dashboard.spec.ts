import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Dashboard } from './dashboard';
import { Device } from '../../api';
import { DevicesStore } from '../../core/devices.store';
import {
  flushRoomOrder,
  flushScenes,
  makeDevice,
  makeLight,
  makeRelay,
  makeScene,
  makeStation,
  testProviders,
} from '../../testing/providers';

describe('Dashboard', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Dashboard],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    flushScenes();
    http.verify();
  });

  /**
   * Remplit le store puis rend le composant.
   *
   * Le tableau de bord ne déclenche pas lui-même le chargement — c'est le
   * composant racine qui s'en charge : il faut donc alimenter le store avant
   * de le rendre.
   */
  async function render(devices: Device[]): Promise<ComponentFixture<Dashboard>> {
    const store = TestBed.inject(DevicesStore);
    store.refresh();

    http.expectOne('/api/devices').flush({ devices, total: devices.length });
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });
    flushRoomOrder();

    const fixture = TestBed.createComponent(Dashboard);
    await fixture.whenStable();
    return fixture;
  }

  function text(fixture: ComponentFixture<Dashboard>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  it("invite à connecter une source quand aucun équipement n'est remonté", async () => {
    const fixture = await render([]);

    expect(text(fixture)).toContain('Aucun équipement');
    const link = (fixture.nativeElement as HTMLElement).querySelector('a[href="/auth/netatmo"]');
    expect(link).not.toBeNull();
  });

  it('groupe les équipements par pièce', async () => {
    const fixture = await render([
      makeDevice({ id: 'a', name: 'Volet salon', room: 'Salon' }),
      makeStation({ id: 'b', name: 'Station', room: 'Bureau' }),
    ]);

    const rooms = [...(fixture.nativeElement as HTMLElement).querySelectorAll('.room-title')];
    expect(rooms.map((el) => el.textContent?.trim())).toEqual(['Bureau', 'Salon']);
  });

  it('affiche les mesures avec leur unité', async () => {
    const fixture = await render([makeStation()]);
    const content = text(fixture);

    expect(content).toContain('21.5 °C');
    expect(content).toContain('48 %');
  });

  it('limite les mesures affichées pour garder les cartes lisibles', async () => {
    // L'état d'une station compte cinq grandeurs ; la carte n'en montre que
    // quatre, le détail restant accessible d'un clic.
    const fixture = await render([makeStation()]);

    const readings = (fixture.nativeElement as HTMLElement).querySelectorAll('.reading');
    expect(readings.length).toBe(4);
  });

  it('signale les équipements injoignables', async () => {
    const fixture = await render([makeDevice({ reachable: false })]);

    expect(text(fixture)).toContain('injoignable');
    expect(text(fixture)).toContain('Hors ligne');
  });

  it('propose les commandes des volets joignables', async () => {
    const fixture = await render([makeDevice()]);
    const content = text(fixture);

    expect(content).toContain('Ouvrir');
    expect(content).toContain('Fermer');
  });

  it('ne propose aucune commande pour une station Netatmo', async () => {
    // L'API météo est en lecture seule : le backend rejetterait la commande.
    const fixture = await render([makeStation()]);

    expect((fixture.nativeElement as HTMLElement).querySelector('.commands')).toBeNull();
  });

  it('ne propose aucune commande pour un volet injoignable', async () => {
    const fixture = await render([makeDevice({ reachable: false })]);

    expect((fixture.nativeElement as HTMLElement).querySelector('.commands')).toBeNull();
  });

  it('envoie la commande du bouton cliqué', async () => {
    const device = makeDevice();
    const fixture = await render([device]);

    const button = (fixture.nativeElement as HTMLElement).querySelector(
      '.commands button',
    ) as HTMLButtonElement;
    button.click();
    await fixture.whenStable();

    // Le premier bouton d'un volet est « Ouvrir ». La carte entière étant un
    // lien vers le détail, le handler stoppe la propagation : si ce n'était
    // pas le cas, la navigation détruirait le composant avant l'envoi.
    const req = http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`);
    expect(req.request.body).toEqual({ command: 'open', parameters: [] });
    req.flush({ exec_id: 'exec-1' });
  });

  it("affiche l'erreur de chargement", async () => {
    const store = TestBed.inject(DevicesStore);
    store.refresh();

    http
      .expectOne('/api/devices')
      .flush(
        { title: 'Internal Server Error', status: 500, detail: 'base indisponible' },
        { status: 500, statusText: 'Internal Server Error' },
      );
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });
    flushRoomOrder();

    const fixture = TestBed.createComponent(Dashboard);
    await fixture.whenStable();

    expect(text(fixture)).toContain('base indisponible');
  });

  it('indique la source de chaque équipement', async () => {
    const fixture = await render([makeDevice(), makeStation(), makeLight(), makeRelay()]);

    const tags = [...(fixture.nativeElement as HTMLElement).querySelectorAll('.source-tag')].map(
      (el) => el.textContent?.trim(),
    );
    expect(tags.sort()).toEqual(['Netatmo', 'Philips Hue', 'Shelly', 'Somfy TaHoma']);
  });

  it("distingue un équipement allumé d'un équipement éteint", async () => {
    const fixture = await render([
      makeLight({ id: 'on', name: 'Allumée', state: '{"on":true,"color":"#ffb35c"}' }),
      makeLight({ id: 'off', name: 'Éteinte', state: '{"on":false,"color":"#ffb35c"}' }),
    ]);

    const cards = [...(fixture.nativeElement as HTMLElement).querySelectorAll('a')];
    const lit = cards.find((a) => a.textContent?.includes('Allumée'))!;
    const dark = cards.find((a) => a.textContent?.includes('Éteinte'))!;

    expect(lit.classList).toContain('lit');
    expect(lit.querySelector<HTMLElement>('.power-icon')?.style.color).toBe('rgb(255, 179, 92)');
    expect(dark.classList).not.toContain('lit');
    expect(dark.querySelector('.power-icon')?.classList).toContain('text-muted');
  });

  it('affiche un bouton par scène demandée sur le tableau de bord, et la lance', async () => {
    const fixture = await render([makeDevice()]);
    flushScenes([
      makeScene({ id: 1, name: 'Soirée' }),
      makeScene({ id: 2, name: 'Réveil', show_on_dashboard: false }),
    ]);
    await fixture.whenStable();

    const buttons = [
      ...(fixture.nativeElement as HTMLElement).querySelectorAll('.scene-button button'),
    ];
    expect(buttons.map((b) => b.textContent?.trim())).toEqual(['Soirée']);

    (buttons[0] as HTMLButtonElement).click();
    const req = http.expectOne('/api/scenes/1/run');
    expect(req.request.method).toBe('POST');
    req.flush(makeScene({ id: 1, running: true }));
  });
});
