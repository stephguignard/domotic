import { DecimalPipe } from '@angular/common';
import { Component, computed, inject, input, signal } from '@angular/core';

import { Device } from '../../api';
import { COLOR_TEMPERATURE_RANGE, lightSettings } from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';

/**
 * Réglages d'une lumière : luminosité, couleur, température de blanc.
 *
 * Seuls les réglages que la lampe sait faire sont proposés. Les champs natifs
 * sont préférés aux composants Optimus : leur événement `change` ne part qu'au
 * relâchement du curseur, clavier compris — une seule commande par geste, là
 * où un suivi continu inonderait le pont.
 */
@Component({
  selector: 'app-light-controls',
  imports: [DecimalPipe],
  templateUrl: './light-controls.html',
})
export class LightControls {
  readonly device = input.required<Device>();

  private readonly store = inject(DevicesStore);

  protected readonly range = COLOR_TEMPERATURE_RANGE;
  protected readonly settings = computed(() => lightSettings(this.device()));

  /**
   * Valeurs affichées pendant le glissement, avant l'envoi. Remises à zéro à
   * chaque changement d'équipement, pour que l'état remonté reprenne la main.
   */
  private readonly dragging = signal<{
    id: string;
    brightness?: number;
    colorTemperature?: number;
  }>({ id: '' });

  protected readonly brightness = computed(() => {
    const d = this.dragging();
    if (d.id === this.device().id && d.brightness !== undefined) {
      return d.brightness;
    }
    // Le pont remonte des décimales (56,92 %) ; le curseur avance par pas
    // entiers, et le navigateur arrondirait de lui-même.
    const current = this.settings().brightness;
    return current === undefined ? undefined : Math.round(current);
  });

  protected readonly colorTemperature = computed(() => {
    const d = this.dragging();
    return d.id === this.device().id && d.colorTemperature !== undefined
      ? d.colorTemperature
      : this.settings().colorTemperature;
  });

  protected preview(field: 'brightness' | 'colorTemperature', event: Event): void {
    const value = Number((event.target as HTMLInputElement).value);
    this.dragging.update((d) => ({
      ...(d.id === this.device().id ? d : {}),
      id: this.device().id,
      [field]: value,
    }));
  }

  protected setBrightness(event: Event): void {
    this.store.sendCommand(this.device(), 'setBrightness', [
      Number((event.target as HTMLInputElement).value),
    ]);
  }

  protected setColor(event: Event): void {
    this.store.sendCommand(this.device(), 'setColor', [(event.target as HTMLInputElement).value]);
  }

  protected setColorTemperature(event: Event): void {
    this.store.sendCommand(this.device(), 'setColorTemperature', [
      Number((event.target as HTMLInputElement).value),
    ]);
  }
}
