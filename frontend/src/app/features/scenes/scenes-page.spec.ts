import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Confirmation, ConfirmationService } from '@openng/optimus-ui/api';

import { ScenesPage } from './scenes-page';
import { SceneEditor, toSpec } from './scene-editor';
import { SceneView } from '../../api';
import { DevicesStore } from '../../core/devices.store';
import {
  flushRefresh,
  makeDevice,
  makeRelay,
  makeScene,
  testProviders,
} from '../../testing/providers';

describe('ScenesPage', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ScenesPage],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
    TestBed.inject(DevicesStore).refresh();
    flushRefresh([makeDevice({ id: 'vol', name: 'Volet', room: 'Salon' }), makeRelay()]);
  });

  afterEach(() => {
    http.match('/api/scenes/resolve').forEach((r) => r.flush({ steps: [] }));
    http.verify();
  });

  async function render(scenes: SceneView[]): Promise<ComponentFixture<ScenesPage>> {
    const fixture = TestBed.createComponent(ScenesPage);
    await fixture.whenStable();
    http
      .expectOne('/api/scenes')
      .flush({ scenes, solar_available: false, time_zone: 'Europe/Zurich' });
    await fixture.whenStable();
    return fixture;
  }

  function el(fixture: ComponentFixture<unknown>): HTMLElement {
    return fixture.nativeElement as HTMLElement;
  }

  it('présente chaque scène, ses étapes, ses horaires et sa prochaine exécution', async () => {
    const fixture = await render([
      makeScene({
        name: 'Soirée',
        steps: [
          {
            type: 'action',
            command: 'close',
            parameters: [],
            targets: { devices: ['vol'], rooms: [], kinds: [] },
          },
          { type: 'wait', wait_minutes: 5 },
        ],
        schedules: [{ enabled: true, days: [1, 2, 3, 4, 5], at: 'time', time: '19:30' }],
        next_run_at: '2026-09-25T17:30:00Z',
        last_status: 'partial',
      }),
    ]);

    const card = el(fixture).querySelector('.scene-card')!.textContent!;
    expect(card).toContain('Fermer · Volet');
    expect(card).toContain('Attendre 5 min');
    expect(card).toContain('En semaine · 19:30');
    expect(card).toContain('Prochaine exécution');
    expect(card).toContain('Partiellement réussie');
  });

  it('demande confirmation avant de lancer une scène qui pilote un relais', async () => {
    let asked: Confirmation | undefined;
    vi.spyOn(TestBed.inject(ConfirmationService), 'confirm').mockImplementation(function (
      this: ConfirmationService,
      c: Confirmation,
    ) {
      asked = c;
      return this;
    });
    const fixture = await render([makeScene({ id: 3, name: 'Eau chaude', touches_relays: true })]);

    el(fixture).querySelector<HTMLButtonElement>('.run-scene button')!.click();
    http.expectNone('/api/scenes/3/run');
    expect(asked?.header).toBe('Lancer « Eau chaude » ?');

    asked!.accept!();
    http.expectOne('/api/scenes/3/run').flush(makeScene({ id: 3, running: true }));
  });

  it("crée une scène depuis l'éditeur, avec aperçu des cibles", async () => {
    const fixture = await render([]);
    el(fixture).querySelector<HTMLButtonElement>('.new-scene button')!.click();
    await fixture.whenStable();

    const editor = fixture.debugElement.query(By.directive(SceneEditor))
      .componentInstance as SceneEditor;
    // Remplir le brouillon comme le ferait le formulaire.
    const draft = (editor as unknown as { draft: Parameters<typeof toSpec>[0] }).draft;
    draft.name = '  Fermeture ';
    draft.steps[0].command = 'close';
    draft.steps[0].targets.rooms = ['Salon'];
    draft.schedules.push({ enabled: true, days: [6, 7, 1], at: 'time', time: '21:00' });

    // L'aperçu interroge le backend, sans rien envoyer aux équipements.
    await new Promise((r) => setTimeout(r, 10));
    const preview = http.expectOne('/api/scenes/resolve');
    expect(preview.request.method).toBe('POST');
    preview.flush({
      steps: [
        { targets: [{ id: 'vol', name: 'Volet', room: 'Salon', kind: 'shutter' }], ignored: [] },
      ],
    });

    document.querySelector<HTMLButtonElement>('.save-scene button')!.click();
    const req = http.expectOne('/api/scenes');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({
      name: 'Fermeture',
      show_on_dashboard: true,
      steps: [
        {
          type: 'action',
          command: 'close',
          parameters: [],
          targets: { devices: [], rooms: ['Salon'], kinds: [] },
        },
      ],
      schedules: [{ enabled: true, days: [1, 6, 7], at: 'time', time: '21:00' }],
    });
    req.flush(makeScene({ id: 9, name: 'Fermeture' }));
    await fixture.whenStable();

    expect(el(fixture).textContent).toContain('Fermeture');
  });

  it("garde l'éditeur ouvert et affiche le refus du backend", async () => {
    const fixture = await render([]);
    el(fixture).querySelector<HTMLButtonElement>('.new-scene button')!.click();
    await fixture.whenStable();

    document.querySelector<HTMLButtonElement>('.save-scene button')!.click();
    http
      .expectOne('/api/scenes')
      .flush(
        { title: 'Unprocessable Entity', status: 422, detail: 'scène invalide : le nom est vide' },
        { status: 422, statusText: 'Unprocessable Entity' },
      );
    await fixture.whenStable();

    expect(document.body.textContent).toContain('scène invalide : le nom est vide');
    expect(document.querySelector('.scene-form')).not.toBeNull();
  });
});

describe('toSpec', () => {
  it("ne garde d'un horaire solaire que les champs utiles", () => {
    const spec = toSpec({
      name: 'Soir',
      show_on_dashboard: false,
      steps: [
        {
          type: 'wait',
          wait_minutes: 5,
          command: 'on',
          targets: { devices: ['x'], rooms: [], kinds: [] },
        },
      ],
      schedules: [
        {
          enabled: true,
          days: [3, 1],
          at: 'sunset',
          offset_minutes: -10,
          not_before: '',
          not_after: '22:00',
          time: '07:00',
        },
      ],
    });
    expect(spec.steps).toEqual([{ type: 'wait', wait_minutes: 5 }]);
    expect(spec.schedules).toEqual([
      { enabled: true, days: [1, 3], at: 'sunset', offset_minutes: -10, not_after: '22:00' },
    ]);
  });
});
