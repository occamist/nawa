export type ToastType = "error" | "success" | "info";

export const TOAST_EVENT = "toast:show";

function show(message: string, type: ToastType) {
  document.dispatchEvent(new CustomEvent(TOAST_EVENT, { detail: { message, type } }));
}

export const toast = {
  error: (message: string) => show(message, "error"),
  success: (message: string) => show(message, "success"),
  info: (message: string) => show(message, "info"),
};
