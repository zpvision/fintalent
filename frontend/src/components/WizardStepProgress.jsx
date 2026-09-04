import { Fragment } from 'react'

export default function WizardStepProgress({ steps, current, onStepChange, ariaLabel = 'Прогресс заполнения' }) {
  return <div className="progress-head wizard-progress-head">
    <div className="wizard-progress-main">
      <div className="wizard-step-track" aria-label={ariaLabel}>
        {steps.map((label, index) => <Fragment key={label}>
          {index > 0 && <i className={`wizard-step-connector${index <= current ? ' completed' : ''}`} aria-hidden="true" />}
          <span className="wizard-step-item">
            <button type="button" className={`wizard-step-node${index < current ? ' completed' : ''}${index === current ? ' current' : ''}`} onClick={() => onStepChange?.(index)} aria-current={index === current ? 'step' : undefined}>{index < current ? '✓' : index + 1}</button>
            <small className={index < current ? 'filled' : ''}>{label}</small>
          </span>
        </Fragment>)}
      </div>
      <div className="wizard-progress-copy"><b>Шаг {current + 1} из {steps.length}</b><strong>{steps[current]}</strong></div>
    </div>
  </div>
}
