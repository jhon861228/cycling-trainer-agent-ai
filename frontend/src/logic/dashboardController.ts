import { showToast } from "../utils/toast.js";

const API_BASE = import.meta.env.PUBLIC_API_BASE;

export interface Workout {
    day: number;
    name: string;
    type: string;
    duration_minutes: number;
    plan_id: string;
    intervals?: any[];
    goal?: string;
    title?: string;
    created_at?: string;
    completed?: boolean;
}

export class DashboardController {
    private static initialized = false;
    private state = {
        userEmail: typeof window !== 'undefined' ? localStorage.getItem("userEmail") : null,
        currentFTP: 250,
        activePlan: [] as Workout[],
        workoutChart: null as any
    };

    private els: any = {};

    constructor() {
        if (typeof window === 'undefined' || DashboardController.initialized) return;
        DashboardController.initialized = true;
        this.initSelectors();
        this.initEventListeners();
        this.init();
    }

    private initSelectors() {
        this.els = {
            loader: document.getElementById("loader"),
            welcome: document.getElementById("welcomeMessage"),
            activeBadge: document.getElementById("activeGoalBadge1"),
            workoutsGrid: document.getElementById("timelineTrack"),
            historyList: document.getElementById("historyList"),
            toggleHistory: document.getElementById("toggleHistory"),
            goalModal: document.getElementById("goalModal"),
            workoutModal: document.getElementById("workoutModal"),
            modalDay: document.getElementById("modalDay"),
            modalTitle: document.getElementById("modalTitle"),
            modalDuration: document.getElementById("modalDuration"),
            modalTypeBadge: document.getElementById("modalTypeBadge"),
            intervalsList: document.getElementById("intervalsList"),
            modalDownloadBtn: document.getElementById("modalDownloadBtn") as HTMLAnchorElement,
            generateBtn: document.getElementById("generateBtn"),
            openGoalModal: document.getElementById("openGoalModal"),
            closeGoalModal: document.getElementById("closeGoalModal"),
            closeModal: document.getElementById("closeModal"),
            historyModal: document.getElementById("historyModal"),
            closeHistoryModal: document.getElementById("closeHistoryModal"),
            historyWorkoutsGrid: document.getElementById("historyWorkoutsGrid"),
            historyModalTitle: document.getElementById("historyModalTitle"),
            historyModalMeta: document.getElementById("historyModalMeta"),
            // Hero elements
            todayHero: document.getElementById("todayHero"),
            heroTitle: document.getElementById("heroTitle"),
            heroDuration: document.getElementById("heroDuration"),
            heroType: document.getElementById("heroType"),
            heroGoBtn: document.getElementById("heroGoBtn"),
            modalTSS: document.getElementById("modalTSS"),
            modalIF: document.getElementById("modalIF"),
            logoutBtn: document.getElementById("logoutBtn")
        };
    }

    private initEventListeners() {
        this.els.openGoalModal?.addEventListener("click", () => this.toggleModal(this.els.goalModal, true));
        this.els.closeGoalModal?.addEventListener("click", () => this.toggleModal(this.els.goalModal, false));
        this.els.closeModal?.addEventListener("click", () => this.toggleModal(this.els.workoutModal, false));
        this.els.closeHistoryModal?.addEventListener("click", () => this.toggleModal(this.els.historyModal, false));

        this.els.toggleHistory?.addEventListener("click", () => {
            const isHidden = this.els.historyList?.classList.toggle("hidden");
            this.els.toggleHistory!.textContent = isHidden ? "Ver historial" : "Ocultar historial";
            if (!isHidden) this.loadHistory();
        });

        this.els.generateBtn?.addEventListener("click", () => this.handleGeneratePlan());
        this.els.logoutBtn?.addEventListener("click", () => {
            localStorage.removeItem("token");
            window.location.href = "/login";
        });
    }

    private toggleModal(modal: HTMLElement | null, show: boolean) {
        if (!modal) return;
        modal.classList.toggle("active", show);
        document.body.style.overflow = show ? "hidden" : "";
    }

    private async init() {
        if (!localStorage.getItem("token")) {
            window.location.href = "/login";
            return;
        }

        try {
            const res = await fetch(`${API_BASE}/profile?email=${this.state.userEmail}`);
            if (res.ok) {
                const data = await res.json();
                this.state.currentFTP = data.ftp;
                if (this.els.welcome) {
                    this.els.welcome.innerHTML = `Niveles calibrados a <span class="text-mono font-bold text-primary">${this.state.currentFTP}W</span>`;
                }
            }
            await this.loadActivePlan();
        } catch (e) {
            console.error("Init error:", e);
        }
    }

    private async loadActivePlan() {
        try {
            this.els.loader?.classList.add("active");
            const res = await fetch(`${API_BASE}/workouts?email=${this.state.userEmail}&history=false`);
            if (res.ok) {
                this.state.activePlan = await res.json();
                this.renderWorkouts(this.state.activePlan);
                this.updateActiveBadge();
                this.renderHero();
            }
        } finally {
            this.els.loader?.classList.remove("active");
        }
    }

    private renderHero() {
        if (!this.els.todayHero) return;

        const start = new Date();
        const todayWorkout = this.state.activePlan.find(w => {
            const d = new Date();
            d.setDate(start.getDate() + w.day - 1);
            return d.toDateString() === new Date().toDateString();
        });

        if (todayWorkout) {
            this.els.todayHero.classList.remove("hidden");
            this.els.heroTitle.textContent = todayWorkout.name;
            this.els.heroDuration.textContent = `${todayWorkout.duration_minutes} min`;
            this.els.heroType.textContent = todayWorkout.type;
            this.els.heroGoBtn.onclick = () => this.openWorkoutDetails(todayWorkout);
        } else {
            this.els.todayHero.classList.add("hidden");
        }
    }

    private updateActiveBadge() {
        if (this.els.activeBadge) {
            const plan = this.state.activePlan[0];
            this.els.activeBadge.textContent = plan?.title || plan?.goal || "Sin objetivo definido";
        }
    }

    private renderWorkouts(workouts: Workout[]) {
        if (!this.els.workoutsGrid) return;
        this.els.workoutsGrid.innerHTML = "";

        if (workouts.length === 0) {
            this.els.workoutsGrid.innerHTML = `<p class="placeholder-text">No tienes un plan activo.</p>`;
            return;
        }

        const start = new Date();
        [...workouts].sort((a, b) => a.day - b.day).forEach(w => {
            const date = new Date(start);
            date.setDate(start.getDate() + w.day - 1);
            const isToday = date.toDateString() === new Date().toDateString();

            const card = document.createElement("div");
            card.className = `workout-card ${isToday ? "is-today" : ""} ${w.completed ? "completed" : ""} type-card-${w.type}`;
            card.style.minWidth = "220px";
            card.style.flex = "0 0 auto";

            // Zone Ribbon Calculation
            const maxPower = Math.max(...(w.intervals?.map(i => i.power || i.on_power) || [0]));
            const zoneClass = this.getZoneClass(maxPower);

            card.innerHTML = `
                <div class="zone-ribbon ${zoneClass}"></div>
                <div class="card-done-toggle ${w.completed ? 'active' : ''}" title="${w.completed ? 'Marcar como pendiente' : 'Marcar como hecho'}">
                    <svg viewBox="0 0 24 24" width="18" height="18"><path fill="currentColor" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
                </div>
                <div class="card-date text-xs uppercase text-muted font-bold">${date.toLocaleDateString("es-ES", { day: "numeric", month: "short" })}</div>
                <div class="card-name font-bold" style="font-size: 1.1rem; margin: 4px 0;">${w.name}</div>
                <div class="card-meta text-mono text-sm" style="color: var(--titanium)">${w.type === "rest" ? "Descanso" : `${w.duration_minutes} min`}</div>
            `;

            // Allow clicking the card for details, but prevent propagation if toggle is clicked
            card.onclick = (e) => {
                const target = e.target as HTMLElement;
                if (target.closest('.card-done-toggle')) {
                    e.stopPropagation();
                    this.toggleWorkoutDone(w);
                    return;
                }
                this.openWorkoutDetails(w);
            };
            this.els.workoutsGrid!.appendChild(card);
        });

        this.scrollToToday();
    }

    private scrollToToday() {
        setTimeout(() => {
            const todayCard = this.els.workoutsGrid?.querySelector(".is-today") as HTMLElement;
            if (todayCard && this.els.workoutsGrid) {
                const scrollLeft = todayCard.offsetLeft - (this.els.workoutsGrid.clientWidth / 2) + (todayCard.clientWidth / 2);
                this.els.workoutsGrid.scrollTo({
                    left: scrollLeft,
                    behavior: "smooth"
                });
            }
        }, 100);
    }

    private getZoneClass(power: number): string {
        if (power <= 0.6) return "zone-bg-rest";
        if (power <= 0.75) return "zone-bg-1";
        if (power <= 0.85) return "zone-bg-2";
        if (power <= 0.95) return "zone-bg-3";
        if (power <= 1.05) return "zone-bg-4";
        return "zone-bg-5";
    }

    private getZoneColor(power: number): string {
        if (power <= 0.6) return "#444444";
        if (power <= 0.75) return "#38bdf8";
        if (power <= 0.85) return "#22c55e";
        if (power <= 0.95) return "#eab308";
        if (power <= 1.05) return "#f97316";
        return "#cf2e2e";
    }

    private openWorkoutDetails(w: Workout) {
        this.els.modalDay!.textContent = `DÍA ${w.day}`;
        this.els.modalTitle!.textContent = w.name;
        this.els.modalDuration!.textContent = `${w.duration_minutes} min`;
        this.els.modalTypeBadge!.textContent = w.type;
        this.els.modalTypeBadge!.className = `type-badge type-${w.type}`;
        // Calculation for TSS & IF
        let totalSeconds = w.duration_minutes * 60;
        let weightedPowerSum = 0;
        w.intervals?.forEach(int => {
            const add = (dur: number, pow: number) => {
                weightedPowerSum += dur * Math.pow(pow, 4); // xPower proxy
            };
            if (int.type === "interval") {
                for (let i = 0; i < int.repeat; i++) {
                    add(int.on_duration, int.on_power);
                    add(int.off_duration, int.off_power);
                }
            } else add(int.duration, int.power);
        });

        const np = Math.pow(weightedPowerSum / totalSeconds, 0.25);
        const if_factor = np; // Since power is already relative to FTP (1.0 = 100% FTP)
        const tss = (totalSeconds * np * if_factor) / 3600 * 100;

        if (this.els.modalTSS) this.els.modalTSS.textContent = Math.round(tss).toString();
        if (this.els.modalIF) this.els.modalIF.textContent = if_factor.toFixed(2);

        this.els.intervalsList!.innerHTML = w.intervals?.map((int: any) => `
            <li style="border-left: 3px solid ${this.getZoneColor(int.power || int.on_power)}; padding-left: 12px;">
                <span class="text-mono">${int.type === "interval" ? `${int.repeat}x ${int.on_duration / 60}'@${Math.round(int.on_power * 100)}%` : `${int.duration / 60}' @ ${Math.round(int.power * 100)}%`}</span>
                <strong style="text-transform: capitalize; font-size: 0.7rem; color: var(--text-muted);">${int.type}</strong>
            </li>
        `).join("") || "<li>Día de descanso</li>";

        this.els.modalDownloadBtn!.style.display = w.type === "rest" ? "none" : "flex";
        this.els.modalDownloadBtn!.href = `${API_BASE}/download-workout?email=${this.state.userEmail}&plan_id=${w.plan_id}&day=${w.day}`;

        this.toggleModal(this.els.workoutModal, true);
        setTimeout(() => this.renderChart(w.intervals || []), 300);
    }

    private renderChart(intervals: any[]) {
        if (this.state.workoutChart) this.state.workoutChart.destroy();
        const canvas = document.getElementById("workoutChart") as HTMLCanvasElement;
        const ctx = canvas?.getContext("2d");
        if (!ctx) return;

        const labels: any[] = [];
        const datasets: any[] = [];

        // Multi-dataset for solid blocks (one dataset per segment for solid discrete coloring)
        let currentTime = 0;

        intervals.forEach((int, idx) => {
            const addSegment = (dur: number, pow: number) => {
                const color = this.getZoneColor(pow);
                datasets.push({
                    label: `Seg ${idx}`,
                    data: [
                        { x: currentTime / 60, y: pow * 100 },
                        { x: (currentTime + dur) / 60, y: pow * 100 }
                    ],
                    borderColor: "transparent",
                    backgroundColor: color,
                    fill: { target: 'origin', above: color },
                    stepped: "before",
                    pointRadius: 0,
                    tension: 0,
                    showLine: true
                });
                currentTime += dur;
            };

            if (int.type === "interval") {
                for (let i = 0; i < int.repeat; i++) {
                    addSegment(int.on_duration, int.on_power);
                    addSegment(int.off_duration, int.off_power);
                }
            } else {
                addSegment(int.duration, int.power);
            }
        });

        // @ts-ignore
        this.state.workoutChart = new Chart(ctx, {
            type: "line",
            data: { datasets: datasets },
            options: {
                responsive: true, maintainAspectRatio: false, animation: false,
                scales: {
                    x: { type: "linear", beginAtZero: true, max: currentTime / 60, ticks: { color: "#888", font: { family: 'JetBrains Mono', size: 10 } }, grid: { display: false } },
                    y: { beginAtZero: true, max: 120, ticks: { color: "#888", font: { family: 'JetBrains Mono', size: 10 }, callback: (v: any) => `${v}%` }, grid: { color: "rgba(255,255,255,0.05)" } }
                },
                plugins: { legend: { display: false }, tooltip: { enabled: false } }
            }
        });
    }

    private async loadHistory() {
        try {
            const res = await fetch(`${API_BASE}/workouts?email=${this.state.userEmail}&history=true`);
            if (res.ok) {
                const data = await res.json();
                this.els.historyList!.innerHTML = "";
                const map = data.reduce((acc: any, w: any) => {
                    if (!acc[w.plan_id]) acc[w.plan_id] = { goal: w.goal, title: w.title, date: w.created_at, days: 0 };
                    acc[w.plan_id].days++;
                    return acc;
                }, {});

                Object.entries(map).forEach(([planId, p]: [string, any]) => {
                    const el = document.createElement("div");
                    el.className = "history-item";
                    el.style.borderBottom = "1px solid var(--glass-border)";
                    el.style.padding = "1rem 0";
                    el.innerHTML = `
                        <div class="history-info">
                            <span class="history-goal font-bold">${p.title || p.goal}</span>
                            <span class="history-meta text-xs text-muted uppercase">${new Date(p.date).toLocaleDateString()} • ${p.days} días</span>
                        </div>
                        <button class="premium-button small" data-id="${planId}">Ver</button>
                    `;
                    el.querySelector("button")?.addEventListener("click", () => {
                        const historyWorkouts = data.filter((w: any) => w.plan_id === planId);
                        this.openHistoryModal(p.title || p.goal, p.date, p.days, historyWorkouts);
                    });
                    this.els.historyList!.appendChild(el);
                });
            }
        } catch (e) {
            console.error("History load error:", e);
        }
    }

    private openHistoryModal(goal: string, date: string, days: number, workouts: Workout[]) {
        if (this.els.historyModalTitle) this.els.historyModalTitle.textContent = goal;
        if (this.els.historyModalMeta) {
            this.els.historyModalMeta.textContent = `${new Date(date).toLocaleDateString()} • ${days} días`;
        }

        if (this.els.historyWorkoutsGrid) {
            this.els.historyWorkoutsGrid.className = "timeline-track"; // Use the same layout
            this.els.historyWorkoutsGrid.innerHTML = "";
            const start = new Date(date);
            [...workouts].sort((a, b) => a.day - b.day).forEach(w => {
                const d = new Date(start);
                d.setDate(start.getDate() + w.day - 1);

                const card = document.createElement("div");
                card.className = `workout-card type-card-${w.type}`;
                card.style.minWidth = "200px";
                card.style.flex = "0 0 auto";

                const maxPower = Math.max(...(w.intervals?.map(i => i.power || i.on_power) || [0]));
                const zoneClass = this.getZoneClass(maxPower);

                card.innerHTML = `
                    <div class="zone-ribbon ${zoneClass}"></div>
                    <div class="card-date text-xs uppercase text-muted font-bold">${d.toLocaleDateString("es-ES", { day: "numeric", month: "short" })}</div>
                    <div class="card-name font-bold" style="margin: 4px 0;">${w.name}</div>
                    <div class="card-meta text-xs" style="color: var(--titanium)">${w.type === "rest" ? "Descanso" : `${w.duration_minutes} min`}</div>
                `;
                card.onclick = () => {
                    this.toggleModal(this.els.historyModal, false);
                    this.openWorkoutDetails(w);
                };
                this.els.historyWorkoutsGrid.appendChild(card);
            });
        }

        this.toggleModal(this.els.historyModal, true);
    }

    private async handleGeneratePlan() {
        const goal = (document.getElementById("goalInput") as HTMLTextAreaElement).value;
        const days = parseInt((document.getElementById("daysInput") as HTMLInputElement).value);
        const availability: any = {};
        document.querySelectorAll(".day-input input").forEach((input: any) => {
            availability[(input as HTMLInputElement).dataset.day!] = parseInt((input as HTMLInputElement).value);
        });

        if (!goal) return showToast("Escribe un objetivo", "error");

        this.els.generateBtn?.setAttribute("disabled", "true");

        try {
            this.els.loader?.classList.add("active");
            const res = await fetch(`${API_BASE}/generate-plan`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ goal, email: this.state.userEmail, days, availability, ftp: this.state.currentFTP }),
            });

            if (res.ok) {
                showToast("¡Plan generado!", "success");
                this.toggleModal(this.els.goalModal, false);
                await this.loadActivePlan();
            } else {
                showToast("Error al generar plan", "error");
            }
        } finally {
            this.els.generateBtn?.removeAttribute("disabled");
            this.els.loader?.classList.remove("active");
        }
    }

    private async toggleWorkoutDone(workout: Workout) {
        this.els.loader?.classList.add("active");
        try {
            const res = await fetch(`${API_BASE}/toggle-workout`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    email: this.state.userEmail,
                    plan_id: workout.plan_id,
                    day: workout.day.toString(),
                    completed: !workout.completed
                })
            });

            if (res.ok) {
                workout.completed = !workout.completed;
                showToast(workout.completed ? "¡Entrenamiento completado!" : "Entrenamiento marcado como pendiente", "success");
                this.renderWorkouts(this.state.activePlan);
                this.renderHero();
            } else {
                showToast("Error al actualizar el estado", "error");
            }
        } catch (e) {
            showToast("Error de conexión", "error");
        } finally {
            this.els.loader?.classList.remove("active");
        }
    }
}
