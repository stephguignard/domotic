import { Injectable, computed, inject, signal } from '@angular/core';
import { ConfirmationService, MessageService } from '@openng/optimus-ui/api';
import { Observable, tap } from 'rxjs';

import { SceneSpec, SceneView, ScenesService } from '../api';
import { describeError } from './errors';

/**
 * État partagé des scènes : la page Scènes les édite, le tableau de bord
 * affiche les boutons de celles qui le demandent.
 */
@Injectable({ providedIn: 'root' })
export class ScenesStore {
  private readonly api = inject(ScenesService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);

  private readonly scenesSignal = signal<SceneView[]>([]);
  private readonly solarSignal = signal(false);
  private readonly timeZoneSignal = signal('');
  private readonly loadedSignal = signal(false);

  /** Scènes, dans l'ordre choisi par l'utilisateur. */
  readonly scenes = this.scenesSignal.asReadonly();
  /** Les horaires solaires sont-ils disponibles ? */
  readonly solarAvailable = this.solarSignal.asReadonly();
  /** Fuseau des heures des horaires. */
  readonly timeZone = this.timeZoneSignal.asReadonly();
  /** Un premier chargement a-t-il abouti ? */
  readonly loaded = this.loadedSignal.asReadonly();

  /** Scènes à afficher en boutons sur le tableau de bord. */
  readonly dashboardScenes = computed(() => this.scenesSignal().filter((s) => s.show_on_dashboard));

  /** Recharge les scènes. */
  load(): void {
    this.api.listScenes().subscribe({
      next: (res) => {
        this.scenesSignal.set(res.scenes);
        this.solarSignal.set(res.solar_available);
        this.timeZoneSignal.set(res.time_zone);
        this.loadedSignal.set(true);
      },
      error: (err: unknown) => {
        this.messages.add({
          severity: 'error',
          summary: 'Scènes',
          detail: describeError(err),
          life: 8000,
        });
      },
    });
  }

  /**
   * Lance une scène. Celle qui pilote un relais — chauffage, eau chaude —
   * demande d'abord confirmation, comme une commande unitaire.
   */
  run(scene: SceneView): void {
    if (!scene.touches_relays) {
      this.doRun(scene);
      return;
    }
    this.confirmation.confirm({
      header: `Lancer « ${scene.name} » ?`,
      message: 'Cette scène agit sur une installation électrique (relais).',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'Lancer',
      rejectLabel: 'Annuler',
      rejectButtonProps: { severity: 'secondary', outlined: true },
      accept: () => this.doRun(scene),
    });
  }

  private doRun(scene: SceneView): void {
    this.api.runScene(scene.id).subscribe({
      next: (updated) => {
        this.replace(updated);
        this.messages.add({ severity: 'success', summary: scene.name, detail: 'Scène lancée' });
        // L'issue se connaît après coup : relire quand les actions sont parties.
        setTimeout(() => this.load(), 4000);
      },
      error: (err: unknown) => {
        this.messages.add({
          severity: 'error',
          summary: scene.name,
          detail: describeError(err),
          life: 8000,
        });
      },
    });
  }

  /**
   * Crée ou modifie une scène. L'erreur est laissée à l'appelant : l'éditeur
   * l'affiche sans se fermer.
   */
  save(spec: SceneSpec, id?: number): Observable<SceneView> {
    const request = id === undefined ? this.api.createScene(spec) : this.api.updateScene(id, spec);
    return request.pipe(
      tap((saved) => {
        this.scenesSignal.update((all) =>
          // Une nouvelle scène prend la dernière place, comme côté backend.
          id === undefined ? [...all, saved] : all.map((s) => (s.id === saved.id ? saved : s)),
        );
        this.messages.add({
          severity: 'success',
          summary: saved.name,
          detail: 'Scène enregistrée',
        });
      }),
    );
  }

  /** Enregistre l'ordre des scènes ; une liste vide revient à l'ordre alphabétique. */
  saveOrder(ids: number[]): void {
    this.api.setSceneOrder({ ids }).subscribe({
      next: (res) => {
        this.scenesSignal.set(res.scenes);
        this.messages.add({
          severity: 'success',
          summary: 'Scènes',
          detail: ids.length
            ? 'Ordre des scènes enregistré'
            : 'Scènes rangées par ordre alphabétique',
        });
      },
      error: (err: unknown) => {
        this.messages.add({
          severity: 'error',
          summary: 'Scènes',
          detail: describeError(err),
          life: 8000,
        });
      },
    });
  }

  /** Supprime une scène, après confirmation. */
  remove(scene: SceneView): void {
    this.confirmation.confirm({
      header: `Supprimer « ${scene.name} » ?`,
      message: 'La scène et ses horaires seront supprimés. Son historique reste consultable.',
      icon: 'pi pi-trash',
      acceptLabel: 'Supprimer',
      rejectLabel: 'Annuler',
      acceptButtonProps: { severity: 'danger' },
      rejectButtonProps: { severity: 'secondary', outlined: true },
      accept: () =>
        this.api.deleteScene(scene.id).subscribe({
          next: () => {
            this.scenesSignal.update((all) => all.filter((s) => s.id !== scene.id));
            this.messages.add({
              severity: 'success',
              summary: scene.name,
              detail: 'Scène supprimée',
            });
          },
          error: (err: unknown) => {
            this.messages.add({
              severity: 'error',
              summary: scene.name,
              detail: describeError(err),
              life: 8000,
            });
          },
        }),
    });
  }

  private replace(updated: SceneView): void {
    this.scenesSignal.update((all) => all.map((s) => (s.id === updated.id ? updated : s)));
  }
}
