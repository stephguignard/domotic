import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CardModule } from '@openng/optimus-ui/card';
import { MessageModule } from '@openng/optimus-ui/message';
import { ProgressSpinnerModule } from '@openng/optimus-ui/progressspinner';
import { TagModule } from '@openng/optimus-ui/tag';

import { Device } from '../../api';
import {
  commandsFor,
  formatValue,
  isControllable,
  kindIcon,
  kindLabel,
  metricLabel,
  litColor,
  litGlow,
  parseState,
  powerState,
  sourceLabel,
  sourceSeverity,
} from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';

/** Une grandeur prête à afficher sur une carte. */
interface Reading {
  label: string;
  value: string;
}

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink, CardModule, ButtonModule, TagModule, MessageModule, ProgressSpinnerModule],
  templateUrl: './dashboard.html',
})
export class Dashboard {
  protected readonly store = inject(DevicesStore);

  protected readonly kindLabel = kindLabel;
  protected readonly kindIcon = kindIcon;
  protected readonly isControllable = isControllable;
  protected readonly commandsFor = commandsFor;
  protected readonly sourceLabel = sourceLabel;
  protected readonly powerState = powerState;
  protected readonly litColor = litColor;
  protected readonly litGlow = litGlow;
  protected readonly sourceSeverity = sourceSeverity;

  /**
   * Grandeurs à afficher sur la carte d'un équipement.
   *
   * L'état complet peut contenir une vingtaine d'entrées — la plupart sans
   * intérêt d'un coup d'œil. En montrer quatre garde les cartes lisibles ; le
   * détail reste accessible d'un clic.
   */
  protected readings(device: Device): Reading[] {
    return Object.entries(parseState(device))
      .filter(([, value]) => value !== null && value !== undefined)
      .slice(0, 4)
      .map(([metric, value]) => ({
        label: metricLabel(metric),
        value: formatValue(metric, value),
      }));
  }

  protected send(event: Event, device: Device, command: string): void {
    // La carte entière est un lien vers le détail : sans cela, agir sur un
    // volet ferait aussi naviguer.
    event.stopPropagation();
    event.preventDefault();
    this.store.sendCommand(device, command);
  }
}
