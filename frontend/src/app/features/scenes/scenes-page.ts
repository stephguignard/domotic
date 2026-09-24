import { DatePipe } from '@angular/common';
import { Component, computed, inject, viewChild } from '@angular/core';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CardModule } from '@openng/optimus-ui/card';
import { TagModule } from '@openng/optimus-ui/tag';

import { DevicesStore } from '../../core/devices.store';
import { describeSchedule, describeStep, sceneStatus } from '../../core/scene-format';
import { ScenesStore } from '../../core/scenes.store';
import { OrderDialog } from '../../shared/order-dialog';
import { SceneEditor } from './scene-editor';

/** Page des scènes : leur contenu, leurs horaires, leur dernière exécution. */
@Component({
  selector: 'app-scenes-page',
  imports: [DatePipe, ButtonModule, CardModule, TagModule, OrderDialog, SceneEditor],
  templateUrl: './scenes-page.html',
})
export class ScenesPage {
  protected readonly scenes = inject(ScenesStore);
  private readonly devices = inject(DevicesStore);

  protected readonly editor = viewChild.required(SceneEditor);

  protected readonly describeSchedule = describeSchedule;
  protected readonly sceneStatus = sceneStatus;

  /** Scènes à ordonner, dans leur ordre actuel. */
  protected readonly orderItems = computed(() =>
    this.scenes.scenes().map((s) => ({ key: s.id, label: s.name })),
  );

  constructor() {
    this.scenes.load();
  }

  protected describeStep = (step: Parameters<typeof describeStep>[0]) =>
    describeStep(step, (id) => this.devices.devices().find((d) => d.id === id)?.name ?? id);
}
