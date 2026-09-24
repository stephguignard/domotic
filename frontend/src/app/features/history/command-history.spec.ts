import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CommandHistory } from './command-history';
import { CommandLogEntry } from '../../api';
import { DevicesStore } from '../../core/devices.store';
import { makeLight, testProviders } from '../../testing/providers';

describe('CommandHistory', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommandHistory],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  function entry(overrides: Partial<CommandLogEntry> = {}): CommandLogEntry {
    return {
      id: 1,
      device_id: 'l-1',
      device_name: 'Plafonnier',
      device_kind: 'light',
      source: 'hue',
      command: 'setBrightness',
      parameters: [40],
      success: true,
      created_at: '2026-09-24T08:00:00Z',
      ...overrides,
    };
  }

  function rows(fixture: ComponentFixture<CommandHistory>): string[] {
    return [...(fixture.nativeElement as HTMLElement).querySelectorAll('.history-row')].map(
      (el) => el.textContent?.replace(/\s+/g, ' ').trim() ?? '',
    );
  }

  it("liste toutes les actions avec l'équipement, sa source et le résultat", async () => {
    const fixture = TestBed.createComponent(CommandHistory);
    await fixture.whenStable();

    http.expectOne('/api/commands?limit=200').flush({
      entries: [
        entry(),
        entry({
          id: 2,
          device_name: 'Eau chaude',
          device_kind: 'switch',
          source: 'shelly',
          command: 'off',
          parameters: [],
          success: false,
          error: 'module injoignable',
        }),
      ],
      total: 2,
    });
    await fixture.whenStable();

    const [first, second] = rows(fixture);
    expect(first).toContain('Plafonnier');
    expect(first).toContain('Philips Hue');
    expect(first).toContain('Luminosité à 40 %');
    expect(first).toContain('Réussie');
    expect(second).toContain('Eau chaude');
    expect(second).toContain('Éteindre');
    expect(second).toContain('Échec');
  });

  it('se relit après chaque commande envoyée', async () => {
    const fixture = TestBed.createComponent(CommandHistory);
    fixture.componentRef.setInput('deviceId', 'l-1');
    await fixture.whenStable();
    http.expectOne((r) => r.url === '/api/commands').flush({ entries: [], total: 0 });
    await fixture.whenStable();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain(
      'Aucune action sur cet équipement',
    );

    const light = makeLight({ id: 'l-1' });
    TestBed.inject(DevicesStore).sendCommand(light, 'on');
    http.expectOne(`/api/devices/l-1/command`).flush({ exec_id: '' });
    await fixture.whenStable();

    http
      .expectOne((r) => r.url === '/api/commands')
      .flush({ entries: [entry({ command: 'on', parameters: [] })], total: 1 });
    await fixture.whenStable();

    // Colonnes équipement et source masquées : elles seraient redondantes.
    const [row] = rows(fixture);
    expect(row).toContain('Allumer');
    expect(row).toContain('Réussie');
    expect(row).not.toContain('Plafonnier');
    expect(row).not.toContain('Philips Hue');
  });
});
