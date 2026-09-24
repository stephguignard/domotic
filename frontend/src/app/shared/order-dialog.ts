import { Component, input, output, signal } from '@angular/core';
import { ButtonModule } from '@openng/optimus-ui/button';
import { DialogModule } from '@openng/optimus-ui/dialog';
import { OrderListModule } from '@openng/optimus-ui/orderlist';

/** Élément à ordonner : une clé stable et un libellé. */
export interface OrderItem<K = string | number> {
  key: K;
  label: string;
}

/**
 * Bouton et dialogue pour réordonner une liste : au glisser-déposer, ou en
 * sélectionnant un élément puis en utilisant les flèches. Partagé par l'ordre
 * des pièces et celui des scènes.
 *
 * La liste travaille sur une copie : rien n'est émis avant « Enregistrer ».
 */
@Component({
  selector: 'app-order-dialog',
  imports: [ButtonModule, DialogModule, OrderListModule],
  templateUrl: './order-dialog.html',
})
export class OrderDialog<K extends string | number = string | number> {
  /** Libellé du bouton qui ouvre le dialogue. */
  readonly buttonLabel = input.required<string>();
  /** Titre du dialogue. */
  readonly header = input.required<string>();
  /** Explication affichée au-dessus de la liste. */
  readonly hint = input('');
  /** Éléments, dans leur ordre actuel ; lus à l'ouverture. */
  readonly items = input.required<OrderItem<K>[]>();

  /** Émis avec les clés dans le nouvel ordre. */
  readonly saved = output<K[]>();
  /** Émis pour revenir à l'ordre alphabétique. */
  readonly reset = output<void>();

  protected readonly visible = signal(false);
  /** Copie de travail, réordonnée sur place par p-orderList. */
  protected draft: OrderItem<K>[] = [];

  protected open(): void {
    this.draft = [...this.items()];
    this.visible.set(true);
  }

  protected save(): void {
    this.saved.emit(this.draft.map((i) => i.key));
    this.visible.set(false);
  }

  protected resetAlphabetical(): void {
    this.reset.emit();
    this.visible.set(false);
  }
}
