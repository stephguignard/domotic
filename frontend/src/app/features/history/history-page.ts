import { Component } from '@angular/core';
import { CardModule } from '@openng/optimus-ui/card';

import { CommandHistory } from './command-history';

/** Page listant toutes les actions envoyées depuis l'interface. */
@Component({
  selector: 'app-history-page',
  imports: [CardModule, CommandHistory],
  template: `
    <p-card>
      <ng-template #header>
        <div class="px-5 pt-5">
          <h1 class="text-[1.375rem] font-semibold">Historique des actions</h1>
          <p class="mt-1 text-sm text-muted">
            Commandes envoyées depuis l'interface, conservées un an. Les actions faites depuis les
            applications des fabricants ou les interrupteurs physiques n'y figurent pas.
          </p>
        </div>
      </ng-template>
      <app-command-history />
    </p-card>
  `,
})
export class HistoryPage {}
