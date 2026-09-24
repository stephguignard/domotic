import { Component, computed, inject } from '@angular/core';

import { DevicesStore } from '../../core/devices.store';
import { OrderDialog } from '../../shared/order-dialog';

/**
 * Choix de l'ordre d'affichage des pièces.
 *
 * Enregistrer classe toutes les pièces affichées, y compris celles qui ne
 * l'étaient pas encore, puisqu'elles occupent une place visible dans la liste.
 */
@Component({
  selector: 'app-room-order',
  imports: [OrderDialog],
  template: `
    <app-order-dialog
      buttonLabel="Organiser les pièces"
      header="Ordre des pièces"
      hint="« Sans pièce » reste en dernier."
      [items]="items()"
      (saved)="store.saveRoomOrder($event)"
      (reset)="store.saveRoomOrder([])"
    />
  `,
})
export class RoomOrder {
  protected readonly store = inject(DevicesStore);

  protected readonly items = computed(() => this.store.rooms().map((r) => ({ key: r, label: r })));
}
