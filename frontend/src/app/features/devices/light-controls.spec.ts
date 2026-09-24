import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { LightControls } from './light-controls';
import { Device } from '../../api';
import { makeLight, testProviders } from '../../testing/providers';

describe('LightControls', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LightControls],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  async function render(device: Device): Promise<ComponentFixture<LightControls>> {
    const fixture = TestBed.createComponent(LightControls);
    fixture.componentRef.setInput('device', device);
    await fixture.whenStable();
    return fixture;
  }

  function input(fixture: ComponentFixture<LightControls>, name: string): HTMLInputElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector(`input[name="${name}"]`);
  }

  /** Simule un réglage : l'utilisateur déplace le curseur, puis le relâche. */
  function adjust(el: HTMLInputElement, value: string, release = true): void {
    el.value = value;
    el.dispatchEvent(new Event('input'));
    if (release) {
      el.dispatchEvent(new Event('change'));
    }
  }

  function expectCommand(device: Device, command: string, parameters: unknown[]): void {
    const req = http.expectOne(`/api/devices/${encodeURIComponent(device.id)}/command`);
    expect(req.request.body).toEqual({ command, parameters });
    req.flush({ exec_id: '' });
  }

  it('propose les trois réglages pour une lampe couleur', async () => {
    const fixture = await render(makeLight());

    expect(input(fixture, 'brightness')?.value).toBe('57');
    expect(input(fixture, 'color_temperature')?.value).toBe('2240');
    expect(input(fixture, 'color')?.value).toBe('#ffb35c');
  });

  it("ne propose rien pour une prise, qui n'a que marche et arrêt", async () => {
    const fixture = await render(makeLight({ name: 'Prise', state: '{"on":false}' }));

    expect((fixture.nativeElement as HTMLElement).querySelectorAll('input').length).toBe(0);
  });

  it("n'envoie la luminosité qu'au relâchement, en affichant la valeur pendant le geste", async () => {
    const light = makeLight();
    const fixture = await render(light);

    adjust(input(fixture, 'brightness')!, '30', false);
    await fixture.whenStable();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('30 %');
    http.expectNone(`/api/devices/${encodeURIComponent(light.id)}/command`);

    adjust(input(fixture, 'brightness')!, '40');
    expectCommand(light, 'setBrightness', [40]);
  });

  it('envoie la couleur et la température de blanc choisies', async () => {
    const light = makeLight();
    const fixture = await render(light);

    adjust(input(fixture, 'color')!, '#00ff00');
    expectCommand(light, 'setColor', ['#00ff00']);

    adjust(input(fixture, 'color_temperature')!, '4000');
    expectCommand(light, 'setColorTemperature', [4000]);
  });

  it('signale une lampe réglée sur une couleur plutôt que sur un blanc', async () => {
    const fixture = await render(
      makeLight({ state: '{"on":true,"color":"#ff0000","color_temperature":null}' }),
    );

    expect((fixture.nativeElement as HTMLElement).textContent).toContain('réglée sur une couleur');
    expect(input(fixture, 'brightness')).toBeNull();
  });
});
