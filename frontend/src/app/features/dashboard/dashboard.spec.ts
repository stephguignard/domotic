import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Dashboard } from './dashboard';
import { Device } from '../../api';
import { DevicesStore } from '../../core/devices.store';
import { makeDevice, makeStation, testProviders } from '../../testing/providers';

describe('Dashboard', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Dashboard],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

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

    const fixture = TestBed.createComponent(Dashboard);
    await fixture.whenStable();
    return fixture;
  }

  function text(fixture: ComponentFixture<Dashboard>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  it('invite à connecter une source quand aucun équipement n\'est remonté', async () => {
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

    http.expectOne('/api/devices').flush(
      { title: 'Internal Server Error', status: 500, detail: 'base indisponible' },
      { status: 500, statusText: 'Internal Server Error' },
    );
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });

    const fixture = TestBed.createComponent(Dashboard);
    await fixture.whenStable();

    expect(text(fixture)).toContain('base indisponible');
  });
});
