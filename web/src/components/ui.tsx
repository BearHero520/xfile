import {
  ChevronDown16Regular,
  Dismiss24Regular,
  SpinnerIos20Regular,
} from "@fluentui/react-icons";
import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";

export function Button({
  variant = "secondary",
  size = "medium",
  icon,
  loading = false,
  children,
  className = "",
  disabled,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger" | "teal";
  size?: "small" | "medium" | "large";
  icon?: React.ReactNode;
  loading?: boolean;
}) {
  return (
    <button
      className={`button button-${variant} button-${size} ${className}`}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? <SpinnerIos20Regular className="spin" /> : icon}
      {children}
    </button>
  );
}

export function IconButton({
  label,
  children,
  variant = "ghost",
  active = false,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  label: string;
  variant?: "ghost" | "surface" | "primary";
  active?: boolean;
}) {
  return (
    <button
      className={`icon-button icon-button-${variant} ${active ? "is-active" : ""} ${className}`}
      aria-label={label}
      aria-pressed={active || undefined}
      title={label}
      {...props}
    >
      {children}
    </button>
  );
}

export function Popover({
  label,
  trigger,
  children,
  align = "right",
  className = "",
}: {
  label: string;
  trigger: (props: { open: boolean; toggle: () => void }) => React.ReactNode;
  children: React.ReactNode;
  align?: "left" | "right";
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const panelId = useId();

  useEffect(() => {
    if (!open) return;
    const closeOnOutside = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", closeOnOutside);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutside);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  return (
    <div className={`popover-root ${className}`} ref={rootRef}>
      <span aria-controls={panelId} aria-expanded={open}>
        {trigger({ open, toggle: () => setOpen((value) => !value) })}
      </span>
      {open && (
        <section
          id={panelId}
          className={`popover-panel popover-${align}`}
          role="dialog"
          aria-label={label}
        >
          {children}
        </section>
      )}
    </div>
  );
}

export function SegmentedControl<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (value: T) => void;
}) {
  return (
    <div className="segmented" role="radiogroup" aria-label={label}>
      {options.map((option) => (
        <button
          type="button"
          role="radio"
          aria-checked={value === option.value}
          className={value === option.value ? "is-active" : ""}
          key={option.value}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function Modal({
  open,
  title,
  description,
  children,
  footer,
  size = "medium",
  className = "",
  bodyClassName = "",
  onClose,
}: {
  open: boolean;
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  size?: "small" | "medium" | "large";
  className?: string;
  bodyClassName?: string;
  onClose: () => void;
}) {
  const backdropRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      const backdrops = document.querySelectorAll(".modal-backdrop");
      if (backdrops.item(backdrops.length - 1) === backdropRef.current)
        onClose();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", close);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", close);
    };
  }, [open, onClose]);
  if (!open) return null;
  return createPortal(
    <div
      ref={backdropRef}
      className="modal-backdrop"
      role="presentation"
      onMouseDown={(event) => event.currentTarget === event.target && onClose()}
    >
      <section
        className={`modal modal-${size} ${className}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header className="modal-header">
          <div>
            <h2>{title}</h2>
            {description && <p>{description}</p>}
          </div>
          <IconButton label="关闭" onClick={onClose}>
            <Dismiss24Regular />
          </IconButton>
        </header>
        <div className={`modal-body ${bodyClassName}`}>{children}</div>
        {footer && <footer className="modal-footer">{footer}</footer>}
      </section>
    </div>,
    document.body,
  );
}

export function Loading({ label = "正在加载" }: { label?: string }) {
  return (
    <div className="loading">
      <SpinnerIos20Regular className="spin" />
      <span>{label}</span>
    </div>
  );
}

export function Empty({
  title,
  description,
}: {
  title: string;
  description?: string;
}) {
  return (
    <div className="empty">
      <div className="empty-mark">XF</div>
      <strong>{title}</strong>
      {description && <p>{description}</p>}
    </div>
  );
}

export function Field({
  label,
  hint,
  error,
  required = false,
  children,
  className = "",
}: {
  label: string;
  hint?: string;
  error?: string;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <label className={`field ${error ? "has-error" : ""} ${className}`}>
      <span>
        {label}
        {required && <b aria-hidden="true">*</b>}
      </span>
      {children}
      {error ? (
        <small className="field-error">{error}</small>
      ) : (
        hint && <small>{hint}</small>
      )}
    </label>
  );
}

export function Select({
  className = "",
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <span className="select-control">
      <select className={className} {...props}>
        {children}
      </select>
      <ChevronDown16Regular aria-hidden="true" />
    </span>
  );
}

export function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (value: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={`switch ${checked ? "is-on" : ""}`}
      onClick={() => onChange(!checked)}
    >
      <span />
    </button>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: React.ReactNode;
  tone?: "success" | "warning" | "danger" | "info" | "neutral";
}) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="page-header">
      <div>
        {eyebrow && <span className="eyebrow">{eyebrow}</span>}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  );
}

export function ErrorBanner({
  error,
  onRetry,
}: {
  error: string;
  onRetry?: () => void;
}) {
  return (
    <div className="error-banner">
      <span>{error}</span>
      {onRetry && (
        <Button variant="ghost" onClick={onRetry}>
          重试
        </Button>
      )}
    </div>
  );
}
