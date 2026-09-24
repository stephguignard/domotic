import { Component, inject, signal } from '@angular/core';
import { ButtonModule } from '@openng/optimus-ui/button';
import { DialogModule } from '@openng/optimus-ui/dialog';
import { OrderListModule } from '@openng/optimus-ui/orderlist';

import { DevicesStore } from '../../core/devices.store';

/**
 * Choix de l'ordre d'affichage des pièces : un bouton qui ouvre la liste des
 * pièces, à réordonner au glisser-déposer ou avec les flèches.
 *
 * La liste travaille sur une copie : rien n'est enregistré avant « Enregistrer ».
 * Toutes les pièces affichées sont alors classées, y compris celles qui ne
 * l'étaient pas encore, puisqu'elles occupent une place visible dans la liste.
 */
@Component({
  selector: 'app-room-order',
  imports: [ButtonModule, DialogModule, OrderListModule],
  templateUrl: './room-order.html',
})
export class RoomOrder {
  protected readonly store = inject(DevicesStore);

  protected readonly visible = signal(false);
  /** Copie de travail, réordonnée sur place par p-orderList. */
  protected draft: string[] = [];

  protected open(): void {
    this.draft = [...this.store.rooms()];
    this.visible.set(true);
  }

  protected save(): void {
    this.store.saveRoomOrder([...this.draft]);
    this.visible.set(false);
  }

  protected resetAlphabetical(): void {
    this.store.saveRoomOrder([]);
    this.visible.set(false);
  }
}
