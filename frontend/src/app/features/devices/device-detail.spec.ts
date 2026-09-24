import { HttpTestingController, TestRequest } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DeviceDetail } from './device-detail';
import { CommandLogEntry, Device, Measurement } from '../../api';
import { makeDevice, makeStation, testProviders } from '../../testing/providers';

describe('DeviceDetail', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DeviceDetail],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  /** Répond aux lectures d'historique des actions, secondaires pour la plupart des cas. */
  function flushHistory(entries: CommandLogEntry[] = []): void {
    http
      .match((req) => req.url === '/api/commands')
      .forEach((req) => req.flush({ entries, total: entries.length }));
  }

  afterEach(() => {
    flushHistory();
    http.verify();
  });

  /** Requête d'historique, dont les paramètres de plage varient à chaque appel. */
  function measurementsRequest(id: string): TestRequest {
    const path = `/api/devices/${encodeURIComponent(id)}/measurements`;
    return http.expectOne((req) => req.url === path);
  }

  /**
   * Rend le composant pour un identifiant donné et répond à ses deux appels.
   *
   * Passer `device` à null simule un équipement introuvable.
   */
  async function render(
    device: Device | null,
    measurements: Measurement[] = [],
  ): Promise<ComponentFixture<DeviceDetail>> {
    const id = device?.id ?? 'inconnu';
    const fixture = TestBed.createComponent(DeviceDetail);

    // `id` est une entrée requise, normalement alimentée par le routeur via
    // withComponentInputBinding().
    fixture.componentRef.setInput('id', id);
    await fixture.whenStable();

    const deviceRequest = http.expectOne(`/api/devices/${encodeURIComponent(id)}`);
    if (device) {
      deviceRequest.flush(device);
    } else {
      deviceRequest.flush(
        { title: 'Not Found', status: 404, detail: 'équipement introuvable' },
        { status: 404, statusText: 'Not Found' },
      );
    }

    measurementsRequest(id).flush({ measurements, total: measurements.length });

    await fixture.whenStable();
    return fixture;
  }

  function text(fixture: ComponentFixture<DeviceDetail>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  function readings(deviceId: string, metric: string, values: number[]): Measurement[] {
    return values.map((value, i) => ({
      device_id: deviceId,
      metric,
      value,
      recorded_at: new Date(Date.UTC(2026, 8, 23, 10 + i)).toISOString(),
    }));
  }

  it("affiche l'identité de l'équipement", async () => {
    const fixture = await render(makeDevice({ name: 'Volet salon', room: 'Salon' }));
    const content = text(fixture);

    expect(content).toContain('Volet salon');
    expect(content).toContain('Volet'); // libellé du type
    expect(content).toContain('Salon');
  });

  it('signale un équipement introuvable', async () => {
    const fixture = await render(null);
    expect(text(fixture)).toContain('introuvable');
  });

  it('liste tous les états courants, sans les tronquer', async () => {
    // Contrairement aux cartes du tableau de bord, le détail montre l'état
    // complet : c'est sa raison d'être.
    const fixture = await render(makeStation());

    const rows = (fixture.nativeElement as HTMLElement).querySelectorAll('.state-row');
    expect(rows.length).toBe(5);
    expect(text(fixture)).toContain('21.5 °C');
  });

  it('indique quand aucun état n\'est remonté', async () => {
    const fixture = await render(makeDevice({ state: '{}' }));
    expect(text(fixture)).toContain('Aucun état remonté');
  });

  it('propose les commandes des équipements pilotables', async () => {
    const fixture = await render(makeDevice());
    const content = text(fixture);

    expect(content).toContain('Ouvrir');
    expect(content).toContain('Stop');
    expect(content).toContain('Fermer');
  });

  it('envoie la commande sélectionnée', async () => {
    const device = makeDevice();
    const fixture = await render(device);

    const buttons = [
      ...(fixture.nativeElement as HTMLElement).querySelectorAll<HTMLButtonElement>(
        '.commands button',
      ),
    ];
    buttons[2].click(); // « Fermer »
    await fixture.whenStable();

    const req = http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`);
    expect(req.request.body).toEqual({ command: 'close', parameters: [] });
    req.flush({ exec_id: 'exec-1' });
  });

  it('ne propose aucune commande pour une station Netatmo', async () => {
    const fixture = await render(makeStation());
    expect((fixture.nativeElement as HTMLElement).querySelector('.commands')).toBeNull();
  });

  it("demande une semaine d'historique", async () => {
    const device = makeStation();
    const fixture = TestBed.createComponent(DeviceDetail);
    fixture.componentRef.setInput('id', device.id);
    await fixture.whenStable();

    http.expectOne(`/api/devices/${encodeURIComponent(device.id)}`).flush(device);

    // Le client généré construit ses paramètres avec sa propre classe
    // OpenApiHttpParams : lire l'URL émise est plus fiable que d'interroger
    // `request.params`.
    const req = measurementsRequest(device.id);
    const params = new URL(req.request.urlWithParams, 'http://localhost').searchParams;

    const from = new Date(params.get('from') as string);
    const days = (Date.now() - from.getTime()) / (24 * 3600 * 1000);

    // Assez pour lire une tendance, assez peu pour que le graphique reste
    // lisible et la réponse légère.
    expect(days).toBeGreaterThan(6.9);
    expect(days).toBeLessThan(7.1);
    expect(params.get('limit')).toBe('2000');

    req.flush({ measurements: [], total: 0 });
    await fixture.whenStable();
  });

  it('présélectionne la première grandeur disponible', async () => {
    const device = makeStation();
    const fixture = await render(device, [
      ...readings(device.id, 'temperature', [20, 21]),
      ...readings(device.id, 'humidity', [50]),
    ]);

    const component = fixture.componentInstance as unknown as {
      selectedMetric(): string | null;
      metricOptions(): { label: string; value: string }[];
    };

    // Les options sont triées par libellé : « Humidité » précède « Température ».
    expect(component.metricOptions().map((o) => o.value)).toEqual(['humidity', 'temperature']);
    expect(component.selectedMetric()).toBe('humidity');
  });

  it('ordonne les points du graphique du plus ancien au plus récent', async () => {
    const device = makeStation();

    // Le backend renvoie du plus récent au plus ancien ; un graphique se lit
    // dans l'autre sens.
    const descending = readings(device.id, 'temperature', [18, 20, 22]).reverse();
    const fixture = await render(device, descending);

    const component = fixture.componentInstance as unknown as {
      chartData(): { datasets: { data: number[] }[] } | null;
    };

    expect(component.chartData()?.datasets[0].data).toEqual([18, 20, 22]);
  });

  it('explique l\'absence d\'historique pour un équipement pilotable', async () => {
    const fixture = await render(makeDevice(), []);

    expect(text(fixture)).toContain('Aucun relevé enregistré');
  });

  it("montre les dernières actions sur l'équipement", async () => {
    const device = makeDevice();
    const fixture = await render(device);

    const req = http.expectOne((r) => r.url === '/api/commands');
    // Le client généré encode lui-même les valeurs ; l'URL ne l'est qu'une fois.
    expect(decodeURIComponent(req.request.params.get('device_id') ?? '')).toBe(device.id);
    req.flush({
      entries: [
        {
          id: 2,
          device_id: device.id,
          device_name: device.name,
          device_kind: 'shutter',
          source: 'tahoma',
          command: 'close',
          parameters: [],
          success: false,
          error: 'box injoignable',
          created_at: '2026-09-24T08:00:00Z',
        },
      ],
      total: 1,
    });
    await fixture.whenStable();

    const row = (fixture.nativeElement as HTMLElement).querySelector('.history-row');
    expect(row?.textContent).toContain('Fermer');
    expect(row?.textContent).toContain('Échec');
  });

  it("n'affiche pas d'historique des actions pour une station Netatmo", async () => {
    const fixture = await render(makeStation());

    http.expectNone((r) => r.url === '/api/commands');
    expect(text(fixture)).not.toContain('Dernières actions');
  });
});
