import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { App } from './app';
import { HealthOutputBody } from './api';
import { flushRoomOrder, testProviders } from './testing/providers';

describe('App', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  /**
   * Crée le composant racine et répond à ses deux appels de démarrage.
   * Passer `health` à null simule un backend injoignable.
   */
  function render(health: Partial<HealthOutputBody> | null): ComponentFixture<App> {
    const fixture = TestBed.createComponent(App);

    http.expectOne('/api/devices').flush({ devices: [], total: 0 });

    flushRoomOrder();
    const healthRequest = http.expectOne('/api/health');
    if (health) {
      healthRequest.flush(health);
    } else {
      healthRequest.error(new ProgressEvent('error'));
    }

    return fixture;
  }

  afterEach(() => http.verify());

  it("crée l'application et charge les données au démarrage", () => {
    // Le composant racine amorce le chargement : sans cela, le tableau de bord
    // s'afficherait vide au premier rendu.
    const fixture = render({ status: 'ok', database: true, sources: {} });
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('signale un service hors ligne quand la santé est inaccessible', async () => {
    const fixture = render(null);
    await fixture.whenStable();

    const badge = fixture.nativeElement.querySelector('p-tag');
    expect(badge?.textContent).toContain('Hors ligne');
  });

  it('signale un service dégradé en nommant la source en échec', async () => {
    const fixture = render({
      status: 'degraded',
      database: true,
      sources: {
        netatmo: { enabled: true, healthy: false, last_error: 'quota dépassé' },
        tahoma: { enabled: false, healthy: false },
      },
    });
    await fixture.whenStable();

    // Nommer la source évite d'avoir à ouvrir /api/health pour diagnostiquer.
    // tahoma est désactivée : elle ne doit pas être présentée comme en échec.
    const badge = fixture.nativeElement.querySelector('p-tag');
    expect(badge?.textContent).toContain('netatmo');
    expect(badge?.textContent).not.toContain('tahoma');
  });
});
