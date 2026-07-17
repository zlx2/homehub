import { createApp } from "vue";

import App from "@/App.vue";
import "@/styles.css";

const root = document.querySelector<HTMLElement>("#app");
if (!root) throw new Error("Drop root element is missing");

createApp(App, {
	page: root.dataset.page || "app",
	role: root.dataset.role || "guest",
}).mount(root);
