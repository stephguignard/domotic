import { Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CheckboxModule } from '@openng/optimus-ui/checkbox';
import { DialogModule } from '@openng/optimus-ui/dialog';
import { InputNumberModule } from '@openng/optimus-ui/inputnumber';
import { InputTextModule } from '@openng/optimus-ui/inputtext';
import { MessageModule } from '@openng/optimus-ui/message';
import { MultiSelectModule } from '@openng/optimus-ui/multiselect';
import { SelectModule } from '@openng/optimus-ui/select';
import { ToggleSwitchModule } from '@openng/optimus-ui/toggleswitch';

import {
  ResolvedStep,
  SceneSchedule,
  SceneSpec,
  SceneStep,
  SceneView,
  ScenesService,
} from '../../api';
import { isControllable } from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';
import { describeError } from '../../core/errors';
import { DAYS, SCENE_ACTIONS, SCENE_KINDS, defaultParameters } from '../../core/scene-format';
import { ScenesStore } from '../../core/scenes.store';

/** Étape en cours d'édition : les cibles sont toujours présentes. */
type DraftStep = SceneStep & { targets: { devices: string[]; rooms: string[]; kinds: string[] } };

/** Scène en cours d'édition, modifiée sur place par les champs du formulaire. */
interface Draft {
  name: string;
  show_on_dashboard: boolean;
  steps: DraftStep[];
  schedules: SceneSchedule[];
}

/**
 * Éditeur de scène : ses étapes, leurs cibles, ses horaires.
 *
 * Chaque modification relance, après une courte pause, la résolution des
 * cibles par le backend : l'aperçu montre les équipements réellement touchés,
 * avec les mêmes règles que l'exécution.
 */
@Component({
  selector: 'app-scene-editor',
  imports: [
    FormsModule,
    ButtonModule,
    CheckboxModule,
    DialogModule,
    InputNumberModule,
    InputTextModule,
    MessageModule,
    MultiSelectModule,
    SelectModule,
    ToggleSwitchModule,
  ],
  templateUrl: './scene-editor.html',
})
export class SceneEditor {
  /** Émis après un enregistrement réussi. */
  readonly saved = output<SceneView>();

  private readonly scenesApi = inject(ScenesService);
  protected readonly scenes = inject(ScenesStore);
  private readonly devices = inject(DevicesStore);

  protected readonly actions = SCENE_ACTIONS;
  protected readonly kinds = SCENE_KINDS;
  protected readonly days = DAYS;

  protected readonly visible = signal(false);
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly preview = signal<ResolvedStep[]>([]);

  protected editedId: number | undefined;
  protected draft: Draft = emptyDraft();

  protected readonly roomOptions = computed(() =>
    this.devices.rooms().map((r) => ({ label: r, value: r })),
  );

  protected readonly deviceOptions = computed(() =>
    this.devices
      .devices()
      .filter((d) => isControllable({ ...d, reachable: true }))
      .map((d) => ({ label: `${d.name} — ${d.room || 'Sans pièce'}`, value: d.id })),
  );

  /**
   * Déclencheurs proposés. Sans coordonnées, les options solaires restent
   * visibles mais désactivées, et disent pourquoi : une option grisée sans
   * explication passe pour une panne.
   */
  protected readonly atOptions = computed(() => {
    const solar = this.scenes.solarAvailable();
    const why = solar ? '' : ' — coordonnées non configurées';
    return [
      { label: 'Heure fixe', value: 'time', disabled: false },
      { label: `Lever du soleil${why}`, value: 'sunrise', disabled: !solar },
      { label: `Coucher du soleil${why}`, value: 'sunset', disabled: !solar },
    ];
  });

  /** Le service compte-t-il les heures en UTC, faute de fuseau configuré ? */
  protected readonly utc = computed(() => ['UTC', 'Etc/UTC'].includes(this.scenes.timeZone()));

  /** La scène pilote-t-elle un relais, d'après l'aperçu ? */
  protected readonly touchesRelays = computed(() =>
    this.preview().some((step) => step.targets.some((t) => t.kind === 'switch')),
  );

  private previewTimer: ReturnType<typeof setTimeout> | undefined;

  /** Ouvre l'éditeur, sur une scène existante ou une nouvelle. */
  open(scene?: SceneView): void {
    this.editedId = scene?.id;
    this.draft = scene ? toDraft(scene) : emptyDraft();
    this.error.set(null);
    this.preview.set([]);
    this.visible.set(true);
    this.refreshPreview(0);
  }

  protected close(): void {
    this.visible.set(false);
  }

  // --- Étapes ------------------------------------------------------------

  protected addAction(): void {
    this.draft.steps.push(newAction());
    this.changed();
  }

  protected addWait(): void {
    this.draft.steps.push({ type: 'wait', wait_minutes: 5, targets: emptyTargets() });
    this.changed();
  }

  protected move(index: number, delta: number): void {
    const to = index + delta;
    if (to < 0 || to >= this.draft.steps.length) return;
    const [step] = this.draft.steps.splice(index, 1);
    this.draft.steps.splice(to, 0, step);
    this.changed();
  }

  protected removeStep(index: number): void {
    this.draft.steps.splice(index, 1);
    this.changed();
  }

  protected commandChanged(step: DraftStep): void {
    step.parameters = defaultParameters(step.command!);
    this.changed();
  }

  protected setColor(step: DraftStep, event: Event): void {
    step.parameters = [(event.target as HTMLInputElement).value];
    this.changed();
  }

  /** Aperçu lisible des cibles d'une étape. */
  protected previewOf(index: number): { targets: string; ignored: string } | null {
    const step = this.draft.steps[index];
    const resolved = this.preview()[index];
    if (step?.type !== 'action' || !resolved) return null;

    const n = resolved.targets.length;
    const targets =
      n === 0
        ? 'Aucun équipement ne recevra cette action.'
        : `${n} équipement${n > 1 ? 's' : ''} : ${resolved.targets.map((t) => t.name).join(', ')}`;
    const ignored = resolved.ignored.map((i) => `${i.name} (${i.reason})`).join(', ');
    return { targets, ignored: ignored ? `Écarté : ${ignored}` : '' };
  }

  // --- Horaires ----------------------------------------------------------

  protected addSchedule(): void {
    this.draft.schedules.push({
      enabled: true,
      days: [1, 2, 3, 4, 5, 6, 7],
      at: 'time',
      time: '07:30',
    });
  }

  protected removeSchedule(index: number): void {
    this.draft.schedules.splice(index, 1);
  }

  protected toggleDay(schedule: SceneSchedule, day: number): void {
    schedule.days = schedule.days.includes(day)
      ? schedule.days.filter((d) => d !== day)
      : [...schedule.days, day].sort((a, b) => a - b);
  }

  protected presetDays(schedule: SceneSchedule, days: number[]): void {
    schedule.days = [...days];
  }

  // --- Enregistrement ----------------------------------------------------

  protected save(): void {
    this.saving.set(true);
    this.error.set(null);
    this.scenes.save(toSpec(this.draft), this.editedId).subscribe({
      next: (scene) => {
        this.saving.set(false);
        this.visible.set(false);
        this.saved.emit(scene);
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.error.set(describeError(err));
      },
    });
  }

  /** À appeler après toute modification des étapes. */
  protected changed(): void {
    this.refreshPreview(300);
  }

  private refreshPreview(delay: number): void {
    clearTimeout(this.previewTimer);
    this.previewTimer = setTimeout(() => {
      const steps = toSpec(this.draft).steps;
      this.scenesApi.resolveSceneSteps({ steps }).subscribe({
        next: (res) => this.preview.set(res.steps),
        error: () => this.preview.set([]),
      });
    }, delay);
  }
}

function emptyTargets() {
  return { devices: [] as string[], rooms: [] as string[], kinds: [] as string[] };
}

function newAction(): DraftStep {
  return { type: 'action', command: 'on', parameters: [], targets: emptyTargets() };
}

function emptyDraft(): Draft {
  return { name: '', show_on_dashboard: true, steps: [newAction()], schedules: [] };
}

/** Copie profonde d'une scène, pour que l'annulation ne laisse aucune trace. */
function toDraft(scene: SceneView): Draft {
  return {
    name: scene.name,
    show_on_dashboard: scene.show_on_dashboard,
    steps: scene.steps.map((s) => ({
      ...s,
      parameters: [...(s.parameters ?? [])],
      targets: {
        devices: [...(s.targets?.devices ?? [])],
        rooms: [...(s.targets?.rooms ?? [])],
        kinds: [...(s.targets?.kinds ?? [])],
      },
    })),
    schedules: scene.schedules.map((s) => ({ ...s, days: [...s.days] })),
  };
}

/** Réduit le brouillon à ce que l'API attend : champs pertinents seulement. */
export function toSpec(draft: Draft): SceneSpec {
  return {
    name: draft.name.trim(),
    show_on_dashboard: draft.show_on_dashboard,
    steps: draft.steps.map((s): SceneStep =>
      s.type === 'wait'
        ? { type: 'wait', wait_minutes: s.wait_minutes ?? 0 }
        : {
            type: 'action',
            command: s.command,
            parameters: s.parameters ?? [],
            targets: s.targets,
          },
    ),
    schedules: draft.schedules.map((s): SceneSchedule => {
      const days = [...s.days].sort((a, b) => a - b);
      if (s.at === 'time') {
        return { enabled: s.enabled, days, at: 'time', time: s.time };
      }
      return {
        enabled: s.enabled,
        days,
        at: s.at,
        offset_minutes: s.offset_minutes ?? 0,
        ...(s.not_before ? { not_before: s.not_before } : {}),
        ...(s.not_after ? { not_after: s.not_after } : {}),
      };
    }),
  };
}
