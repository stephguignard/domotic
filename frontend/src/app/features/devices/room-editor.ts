import { Component, ElementRef, effect, inject, input, signal, viewChild } from '@angular/core';
import { ButtonModule } from '@openng/optimus-ui/button';

import { Device } from '../../api';
import { DevicesStore } from '../../core/devices.store';

/**
 * Pièce d'un équipement, modifiable sur place.
 *
 * La saisie propose les pièces existantes (datalist) sans interdire d'en
 * créer une : une pièce n'existe que par les équipements qu'on y range.
 */
@Component({
  selector: 'app-room-editor',
  imports: [ButtonModule],
  templateUrl: './room-editor.html',
})
export class RoomEditor {
  readonly device = input.required<Device>();

  protected readonly store = inject(DevicesStore);
  protected readonly editing = signal(false);

  private readonly roomInput = viewChild<ElementRef<HTMLInputElement>>('room');

  constructor() {
    // `autofocus` n'agit pas sur un champ inséré après le chargement de la page.
    effect(() => this.roomInput()?.nativeElement.select());
  }

  protected save(value: string): void {
    const room = value.trim();
    this.editing.set(false);
    if (room !== this.device().room || !this.device().room_overridden) {
      this.store.setRoom(this.device(), room);
    }
  }

  protected reset(): void {
    this.editing.set(false);
    this.store.setRoom(this.device(), null);
  }
}
