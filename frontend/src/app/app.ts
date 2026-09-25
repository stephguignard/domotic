import { Component, computed, inject } from '@angular/core';
import { RouterLink, RouterOutlet } from '@angular/router';
import { MenuItem } from '@openng/optimus-ui/api';
import { ButtonModule } from '@openng/optimus-ui/button';
import { ConfirmDialogModule } from '@openng/optimus-ui/confirmdialog';
import { TagModule } from '@openng/optimus-ui/tag';
import { ToastModule } from '@openng/optimus-ui/toast';
import { MenubarModule } from '@openng/optimus-ui/menubar';

import { DevicesStore } from './core/devices.store';

@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    RouterLink,
    MenubarModule,
    ButtonModule,
    ConfirmDialogModule,
    TagModule,
    ToastModule,
  ],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {
  protected readonly store = inject(DevicesStore);

  /**
   * Navigation. Sur un écran étroit, le menubar la replie derrière un bouton
   * hamburger ; un lien choisi referme le menu.
   */
  protected readonly menu: MenuItem[] = [
    { label: 'Tableau de bord', icon: 'pi pi-th-large', routerLink: '/dashboard' },
    { label: 'Équipements', icon: 'pi pi-list', routerLink: '/devices' },
    { label: 'Scènes', icon: 'pi pi-play-circle', routerLink: '/scenes' },
    { label: 'Historique', icon: 'pi pi-history', routerLink: '/history' },
  ];

  /** Résumé de l'état du service, affiché en permanence dans la barre. */
  protected readonly healthTag = computed(() => {
    const health = this.store.health();
    if (!health) {
      return { label: 'Hors ligne', severity: 'danger' as const };
    }
    if (health.status === 'ok') {
      return { label: 'Opérationnel', severity: 'success' as const };
    }

    // Nommer les sources en échec évite d'avoir à ouvrir /api/health pour
    // comprendre ce qui ne va pas.
    const failing = Object.entries(health.sources)
      .filter(([, s]) => s.enabled && !s.healthy)
      .map(([name]) => name);

    return {
      label: failing.length ? `Dégradé : ${failing.join(', ')}` : 'Dégradé',
      severity: 'warn' as const,
    };
  });

  constructor() {
    this.store.refresh();
  }
}
