import { readonly, ref } from "vue";

export type ConfirmTone = "danger" | "default";

export interface ConfirmOptions {
  title: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  tone?: ConfirmTone;
}

interface ConfirmState extends Required<ConfirmOptions> {
  open: boolean;
}

const state = ref<ConfirmState>({
  open: false,
  title: "",
  message: "",
  confirmText: "确定",
  cancelText: "取消",
  tone: "default",
});

let activeResolver: ((value: boolean) => void) | null = null;

const closeConfirm = (value: boolean) => {
  activeResolver?.(value);
  activeResolver = null;
  state.value = {
    ...state.value,
    open: false,
  };
};

const confirmAction = (options: ConfirmOptions) => {
  if (activeResolver) {
    activeResolver(false);
    activeResolver = null;
  }

  state.value = {
    open: true,
    title: options.title,
    message: options.message ?? "",
    confirmText: options.confirmText ?? "确定",
    cancelText: options.cancelText ?? "取消",
    tone: options.tone ?? "default",
  };

  return new Promise<boolean>((resolve) => {
    activeResolver = resolve;
  });
};

export const useConfirm = () => ({
  confirmState: readonly(state),
  confirmAction,
  acceptConfirm: () => closeConfirm(true),
  cancelConfirm: () => closeConfirm(false),
});
