// Cadence · Log Dose — canvas assembly + tweaks

const TWEAK_DEFAULTS_LOG = /*EDITMODE-BEGIN*/{
  "dark": false,
  "offProtocol": false,
  "editPast": false
}/*EDITMODE-END*/;

const LOG_VARIANTS = [
  { id: 'v1-wizard',   title: 'Cadence · Log dose',  subtitle: 'Five-step wizard with body diagram for site selection — warm sheet confirmation.', Comp: LogDoseV1 },
];

function LogDoseFrame({ Comp, dark, offProtocol, editPast }) {
  return (
    <IOSDevice width={390} height={844} dark={dark}>
      <div style={{ height: '100%', position: 'relative' }}>
        <Comp dark={dark} offProtocol={offProtocol} editPast={editPast} />
      </div>
    </IOSDevice>
  );
}

function LogDoseApp() {
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS_LOG);

  return (
    <>
      <style>{`
        html, body { background: ${t.dark ? '#0a1610' : '#e7dfcc'}; }
      `}</style>

      <DesignCanvas>
        {LOG_VARIANTS.map(v => (
          <DCSection key={v.id} id={v.id} title={v.title} subtitle={v.subtitle}>
            <DCArtboard id={v.id + '-ios'} label="iOS · iPhone 15" width={390} height={844}>
              <LogDoseFrame Comp={v.Comp} dark={t.dark} offProtocol={t.offProtocol} editPast={t.editPast} />
            </DCArtboard>
          </DCSection>
        ))}
      </DesignCanvas>

      <TweaksPanel>
        <TweakSection label="Flow state" />
        <TweakToggle
          label="Off-protocol dose"
          value={t.offProtocol}
          onChange={(v) => setTweak('offProtocol', v)}
        />
        <TweakToggle
          label="Editing a past dose"
          value={t.editPast}
          onChange={(v) => setTweak('editPast', v)}
        />
        <TweakSection label="Theme" />
        <TweakToggle
          label="Dark mode"
          value={t.dark}
          onChange={(v) => setTweak('dark', v)}
        />
      </TweaksPanel>
    </>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<LogDoseApp />);
