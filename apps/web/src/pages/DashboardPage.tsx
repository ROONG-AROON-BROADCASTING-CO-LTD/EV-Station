import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  ArrowRight,
  Check,
  ChevronRight,
  CircleAlert,
  FileText,
  MapPin,
  MoreHorizontal,
  Pencil,
  Phone,
  Plus,
  Search,
  SlidersHorizontal,
  Trash2,
  TrendingUp,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { MapPanel } from "../components/MapPanel";
import { ErrorState, LoadingState } from "../components/PageState";
import { useI18n } from "../i18n/I18nProvider";
import { api } from "../services/api";
import type { AnalysisRun, Site, UserRole } from "../types/domain";

type WorkflowStage = "submitted" | "documents" | "analysing" | "ready";

const WORKFLOW: Array<{
  id: WorkflowStage;
  icon: typeof Check;
  th: string;
  en: string;
  tone: string;
  activeTone: string;
}> = [
  {
    id: "submitted",
    icon: Check,
    th: "ส่งข้อมูลแล้ว",
    en: "Submitted",
    tone: "text-emerald-600",
    activeTone: "bg-emerald-50 ring-emerald-200",
  },
  {
    id: "documents",
    icon: FileText,
    th: "ต้องตรวจเพิ่ม",
    en: "Needs review",
    tone: "text-orange-500",
    activeTone: "bg-orange-50 ring-orange-200",
  },
  {
    id: "analysing",
    icon: TrendingUp,
    th: "กำลังวิเคราะห์",
    en: "Analysing",
    tone: "text-blue-600",
    activeTone: "bg-blue-50 ring-blue-200",
  },
  {
    id: "ready",
    icon: Phone,
    th: "พร้อมติดตาม",
    en: "Ready to follow up",
    tone: "text-emerald-600",
    activeTone: "bg-emerald-50 ring-emerald-200",
  },
];

function stageFor(site: Site, run?: AnalysisRun | null): WorkflowStage {
  if (site.inputStatus === "missing") return "documents";
  if (!run) return "submitted";
  if (run.status === "pending" || run.status === "running") return "analysing";
  if (run.status === "failed") return "documents";
  return "ready";
}

function copy(language: "th" | "en", th: string, en: string) {
  return language === "th" ? th : en;
}

function formatDate(value: string, language: "th" | "en") {
  return new Date(value).toLocaleDateString(
    language === "th" ? "th-TH" : "en-GB",
    {
      day: "numeric",
      month: "short",
      year: language === "th" ? "numeric" : "2-digit",
    },
  );
}

function locationText(site: Site, language: "th" | "en") {
  if (site.address) return site.address;
  if (site.latitude !== undefined && site.longitude !== undefined)
    return `${site.latitude.toFixed(5)}, ${site.longitude.toFixed(5)}`;
  return copy(language, "ยังไม่มีตำแหน่ง", "Location pending");
}

export function DashboardPage({ role }: { role: UserRole }) {
  const { language } = useI18n();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | WorkflowStage>("all");
  const [sortBy, setSortBy] = useState<"latest" | "score">("latest");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const queryClient = useQueryClient();
  const sites = useQuery({ queryKey: ["sites"], queryFn: api.listSites });
  const deleteSite = useMutation({
    mutationFn: api.deleteSite,
    onSuccess: async () => {
      setSelectedId(null);
      await queryClient.invalidateQueries({ queryKey: ["sites"] });
    },
  });
  const latestAnalyses = useQueries({
    queries: (sites.data ?? []).map((site) => ({
      queryKey: ["latest-analysis", site.id],
      queryFn: () => api.getLatestAnalysisForSite(site.id),
      staleTime: 60_000,
    })),
  });
  const analysisBySite = useMemo(
    () =>
      new Map(
        (sites.data ?? []).map(
          (site, index) => [site.id, latestAnalyses[index]?.data] as const,
        ),
      ),
    [latestAnalyses, sites.data],
  );
  const siteList = sites.data ?? [];
  const stageCounts = useMemo(
    () =>
      siteList.reduce<Record<WorkflowStage, number>>(
        (counts, site) => {
          counts[stageFor(site, analysisBySite.get(site.id))] += 1;
          return counts;
        },
        { submitted: 0, documents: 0, analysing: 0, ready: 0 },
      ),
    [analysisBySite, siteList],
  );
  const filteredSites = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    return [...siteList]
      .filter(
        (site) =>
          filter === "all" ||
          stageFor(site, analysisBySite.get(site.id)) === filter,
      )
      .filter(
        (site) =>
          !normalized ||
          [site.name, site.contactName, site.contactPhone, site.address]
            .filter(Boolean)
            .join(" ")
            .toLocaleLowerCase()
            .includes(normalized),
      )
      .sort((a, b) =>
        sortBy === "score"
          ? (analysisBySite.get(b.id)?.overallScore ?? -1) -
            (analysisBySite.get(a.id)?.overallScore ?? -1)
          : new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
      );
  }, [analysisBySite, filter, query, siteList, sortBy]);
  const selectedSite =
    filteredSites.find((site) => site.id === selectedId) ?? filteredSites[0];
  const selectedAnalysis = selectedSite
    ? analysisBySite.get(selectedSite.id)
    : undefined;
  const isAdmin = role === "admin" || role === "super_admin";
  const title = "Portfolio";
  const subtitle = copy(
    language,
    "ติดตามโอกาสของพื้นที่ ตั้งแต่ส่งข้อมูลจนพร้อมเปิดให้บริการ",
    "Track site opportunities from submission to delivery, across all markets.",
  );

  if (role === "customer")
    return (
      <CustomerDashboard
        language={language}
        sites={siteList}
        analyses={analysisBySite}
        isLoading={sites.isLoading}
        isError={sites.isError}
        error={sites.error}
      />
    );

  return (
    <div className="portfolio-workspace space-y-4 sm:space-y-6">
      <header className="flex items-start justify-between gap-3 sm:flex-wrap sm:gap-5">
        <div className="min-w-0">
          <p className="hidden text-[11px] font-extrabold uppercase tracking-[0.18em] text-slate-500 sm:block">
            Sites
          </p>
          <h1 className="text-2xl font-extrabold tracking-tight text-slate-950 sm:mt-1 sm:text-[40px]">
            {title}
          </h1>
          <p className="mt-1 hidden text-sm text-slate-500 sm:mt-1 sm:block">
            {subtitle}
          </p>
          <p className="mt-1 text-sm text-slate-500 sm:hidden">
            {siteList.length} {copy(language, "รายการในระบบ", "sites")}
          </p>
        </div>
        <Link
          to="/sites/new"
          className="inline-flex min-h-11 shrink-0 items-center gap-2 rounded-md bg-[#009b5a] px-4 text-sm font-bold text-white shadow-[0_7px_18px_rgba(0,155,90,0.2)] transition hover:bg-[#00854d]"
        >
          <Plus size={18} />
          <span className="hidden sm:inline">
            {copy(language, "เพิ่มพื้นที่", "Add site")}
          </span>
          <span className="sm:hidden">{copy(language, "เพิ่ม", "Add")}</span>
        </Link>
      </header>
      <section className="hidden rounded-md border border-slate-200 bg-white p-2 shadow-[0_8px_28px_rgba(15,35,70,0.04)] sm:block">
        <div className="grid divide-y divide-slate-100 sm:grid-cols-2 sm:divide-x sm:divide-y-0 xl:grid-cols-4">
          {WORKFLOW.map((item, index) => {
            const Icon = item.icon;
            const isFiltered = filter === item.id;
            const iconTone =
              item.id === "documents"
                ? "bg-orange-50 text-orange-600"
                : item.id === "analysing"
                  ? "bg-blue-50 text-blue-600"
                  : "bg-emerald-50 text-emerald-700";
            return (
              <button
                key={item.id}
                type="button"
                onClick={() =>
                  setFilter((current) =>
                    current === item.id ? "all" : item.id,
                  )
                }
                className={`group relative flex items-start gap-3 px-4 py-4 text-left transition hover:bg-slate-50 ${isFiltered ? item.activeTone : ""}`}
              >
                <span
                  className={`workflow-stage-icon grid h-9 w-9 shrink-0 place-items-center rounded-full ${iconTone}`}
                >
                  <Icon size={18} strokeWidth={2} />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-bold text-[#0b2851]">
                    {copy(language, item.th, item.en)}
                  </span>
                  <strong className="mt-0.5 block text-sm text-slate-900">
                    {stageCounts[item.id]}
                  </strong>
                  <span className="mt-0.5 block text-xs text-slate-500">
                    {index === 0
                      ? copy(
                          language,
                          "พื้นที่ส่งเข้ามาใหม่",
                          "New site proposals",
                        )
                      : index === 1
                        ? copy(
                            language,
                            "กำลังประเมินความเป็นไปได้",
                            "Assessing feasibility",
                          )
                        : index === 2
                          ? copy(
                              language,
                              "ด้านเทคนิคและธุรกิจ",
                              "Technical and commercial",
                            )
                          : copy(
                              language,
                              "พร้อมดำเนินการต่อ",
                              "Ready to advance",
                            )}
                  </span>
                </span>
                {index < WORKFLOW.length - 1 && (
                  <ChevronRight
                    className="ml-auto mt-3 hidden shrink-0 text-slate-300 xl:block"
                    size={20}
                  />
                )}
                <span
                  className={`absolute bottom-0 left-4 right-4 h-0.5 rounded-full ${item.id === "documents" ? "bg-blue-500" : item.id === "analysing" ? "bg-slate-300" : "bg-emerald-500"} ${isFiltered ? "opacity-100" : "opacity-75"}`}
                />
              </button>
            );
          })}
        </div>
      </section>
      <section className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_350px]">
        <div className="min-w-0 overflow-hidden rounded-md border border-slate-200 bg-white shadow-[0_8px_28px_rgba(15,35,70,0.04)]">
          <div className="grid gap-2 border-b border-slate-200 p-3 sm:flex sm:flex-wrap sm:gap-3 sm:p-4">
            <label className="relative min-w-0 flex-[1.4]">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
                size={18}
              />
              <input
                aria-label={copy(language, "ค้นหาพื้นที่", "Search sites")}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={copy(
                  language,
                  "ค้นหาพื้นที่...",
                  "Search sites...",
                )}
                className="h-11 w-full rounded-md border border-slate-200 bg-white pl-10 pr-3 text-sm outline-none transition placeholder:text-slate-400 focus:border-emerald-500 focus:ring-4 focus:ring-emerald-50 sm:h-10"
              />
            </label>
            <label className="hidden lg:block">
              <span className="sr-only">
                {copy(language, "สถานะ", "Status")}
              </span>
              <select
                value={filter}
                onChange={(event) =>
                  setFilter(event.target.value as "all" | WorkflowStage)
                }
                className="h-10 rounded-md border border-slate-200 bg-white px-3 pr-8 text-sm font-semibold text-[#0b2851] outline-none focus:border-emerald-500"
              >
                <option value="all">{copy(language, "สถานะ", "Status")}</option>
                {WORKFLOW.map((item) => (
                  <option key={item.id} value={item.id}>
                    {copy(language, item.th, item.en)}
                  </option>
                ))}
              </select>
            </label>
            <div className="grid grid-cols-2 gap-2 sm:contents">
              <label>
                <span className="sr-only">
                  {copy(language, "เรียงลำดับ", "Sort")}
                </span>
                <select
                  value={sortBy}
                  onChange={(event) =>
                    setSortBy(event.target.value as "latest" | "score")
                  }
                  className="h-11 w-full rounded-md border border-slate-200 bg-white px-3 pr-8 text-sm font-semibold text-slate-600 outline-none focus:border-emerald-500 sm:h-10 sm:w-auto"
                >
                  <option value="latest">
                    {copy(language, "อัปเดตล่าสุด", "Updated (newest)")}
                  </option>
                  <option value="score">
                    {copy(language, "คะแนนสูงสุด", "Highest score")}
                  </option>
                </select>
              </label>
              <label className="sm:hidden">
                <span className="sr-only">
                  {copy(language, "สถานะ", "Status")}
                </span>
                <select
                  value={filter}
                  onChange={(event) =>
                    setFilter(event.target.value as "all" | WorkflowStage)
                  }
                  className="h-11 w-full rounded-md border border-slate-200 bg-white px-3 pr-8 text-sm font-semibold text-slate-600 outline-none focus:border-emerald-500"
                >
                  <option value="all">
                    {copy(language, "ทุกสถานะ", "All status")}
                  </option>
                  {WORKFLOW.map((item) => (
                    <option key={item.id} value={item.id}>
                      {copy(language, item.th, item.en)}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {(query || filter !== "all" || sortBy !== "latest") && (
              <button
                type="button"
                onClick={() => {
                  setQuery("");
                  setFilter("all");
                  setSortBy("latest");
                }}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-slate-200 px-3 text-sm font-bold text-slate-600 transition hover:border-emerald-300 hover:text-emerald-700"
              >
                <SlidersHorizontal size={16} />
                {copy(language, "ล้าง", "Clear")}
              </button>
            )}
          </div>
          {sites.isLoading && (
            <LoadingState
              label={copy(
                language,
                "กำลังโหลดข้อมูลพื้นที่…",
                "Loading site data…",
              )}
            />
          )}
          {sites.isError && (
            <div className="p-6">
              <ErrorState error={sites.error} />
            </div>
          )}
          {!sites.isLoading && !sites.isError && siteList.length === 0 && (
            <EmptyState language={language} />
          )}{" "}
          {!!siteList.length && (
            <>
              <div className="hidden overflow-x-auto lg:block">
                <div className="flex items-center justify-between px-4 py-3 text-xs">
                  <span className="font-bold text-[#0b2851]">
                    {filteredSites.length} {copy(language, "พื้นที่", "sites")}
                  </span>
                  <span className="text-slate-500">
                    {copy(language, "เรียงตาม", "Sort by")}{" "}
                    <strong className="ml-1 font-bold text-[#0b2851]">
                      {copy(
                        language,
                        sortBy === "latest" ? "อัปเดตล่าสุด" : "คะแนนสูงสุด",
                        sortBy === "latest"
                          ? "Updated (newest)"
                          : "Highest score",
                      )}
                    </strong>
                  </span>
                </div>
                <table className="w-full min-w-[880px] text-left">
                  <thead className="border-y border-slate-200 bg-slate-50 text-[11px] font-extrabold tracking-wide text-[#334155]">
                    <tr>
                      <th className="w-10 px-4 py-3">
                        <span className="block h-4 w-4 rounded border border-slate-300 bg-white" />
                      </th>
                      <th className="px-2 py-3">
                        {copy(language, "ชื่อพื้นที่", "Site name")}
                      </th>
                      <th className="px-3 py-3">
                        {copy(language, "ตำแหน่ง", "Location")}
                      </th>
                      <th className="px-3 py-3">
                        {copy(language, "ผู้ติดต่อ", "Franchisee")}
                      </th>
                      <th className="px-3 py-3">
                        {copy(language, "สถานะ", "Status")}
                      </th>
                      <th className="px-3 py-3">
                        {copy(language, "ความพร้อม", "Readiness")}
                      </th>
                      <th className="px-4 py-3">
                        {copy(language, "อัปเดต", "Updated")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredSites.map((site) => (
                      <SiteRow
                        key={site.id}
                        site={site}
                        run={analysisBySite.get(site.id)}
                        selected={selectedSite?.id === site.id}
                        language={language}
                        onSelect={() => setSelectedId(site.id)}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="divide-y divide-slate-100 lg:hidden">
                {filteredSites.map((site) => (
                  <MobileSiteCard
                    key={site.id}
                    site={site}
                    run={analysisBySite.get(site.id)}
                    language={language}
                  />
                ))}
              </div>
              {!filteredSites.length && (
                <div className="grid min-h-52 place-items-center px-6 text-center">
                  <div>
                    <Search className="mx-auto text-slate-300" size={28} />
                    <p className="mt-3 font-bold text-[#0b2851]">
                      {copy(
                        language,
                        "ไม่พบข้อมูลตามตัวกรอง",
                        "No matching sites",
                      )}
                    </p>
                    <button
                      type="button"
                      onClick={() => {
                        setQuery("");
                        setFilter("all");
                      }}
                      className="mt-2 text-sm font-bold text-emerald-700 hover:underline"
                    >
                      {copy(language, "แสดงทั้งหมด", "Show all")}
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
        <div className="hidden xl:block">
          <SiteInspector
            site={selectedSite}
            run={selectedAnalysis}
            language={language}
            role={role}
            deleting={deleteSite.isPending}
            onDelete={() => {
              if (
                selectedSite &&
                window.confirm(
                  copy(
                    language,
                    "ลบพื้นที่นี้และผลวิเคราะห์ทั้งหมดอย่างถาวรใช่หรือไม่?",
                    "Delete this site and all of its analysis results permanently?",
                  ),
                )
              )
                deleteSite.mutate(selectedSite.id);
            }}
          />
        </div>
      </section>
    </div>
  );
}

function CustomerDashboard({
  language,
  sites,
  analyses,
  isLoading,
  isError,
  error,
}: {
  language: "th" | "en";
  sites: Site[];
  analyses: Map<string, AnalysisRun | null | undefined>;
  isLoading: boolean;
  isError: boolean;
  error: unknown;
}) {
  const title = copy(language, "ข้อมูลพื้นที่ของคุณ", "Your site information");
  const subtitle = copy(
    language,
    "ติดตามสถานะพื้นที่ที่คุณส่งให้ทีม RBC ตรวจสอบ",
    "Track the sites you submitted for the RBC team to review.",
  );
  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-5">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-[#08244d] sm:text-[38px]">
            {title}
          </h1>
          <p className="mt-2 text-sm text-slate-500">{subtitle}</p>
        </div>
        <Link
          to="/sites/new"
          className="inline-flex min-h-11 items-center gap-2 rounded-lg bg-[#007070] px-5 text-sm font-bold text-white shadow-[0_7px_18px_rgba(0,112,112,0.2)] transition hover:bg-[#005b5b]"
        >
          <Plus size={19} />
          {copy(language, "ส่งข้อมูลพื้นที่", "Submit site")}
        </Link>
      </header>
      <section className="rounded-xl border border-emerald-100 bg-emerald-50/70 px-5 py-4">
        <div className="flex items-start gap-3">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-white text-emerald-700 shadow-sm">
            <Check size={18} />
          </span>
          <div>
            <p className="font-bold text-emerald-900">
              {copy(
                language,
                "ส่งข้อมูลแล้ว ทีมงานจะดำเนินการวิเคราะห์ให้",
                "Your information is submitted. Our team will perform the analysis.",
              )}
            </p>
            <p className="mt-1 text-sm leading-6 text-emerald-800">
              {copy(
                language,
                "คุณสามารถดูสถานะและผลวิเคราะห์ที่ทีมจัดทำเสร็จแล้วได้จากหน้านี้",
                "You can view the status and the completed screening result here.",
              )}
            </p>
          </div>
        </div>
      </section>
      {isLoading && (
        <LoadingState
          label={copy(
            language,
            "กำลังโหลดข้อมูลพื้นที่…",
            "Loading site data…",
          )}
        />
      )}
      {isError && <ErrorState error={error} />}
      {!isLoading && !isError && !sites.length && (
        <EmptyState language={language} />
      )}
      <div className="grid gap-5 md:grid-cols-2">
        {sites.map((site) => (
          <CustomerSiteCard
            key={site.id}
            site={site}
            run={analyses.get(site.id)}
            language={language}
          />
        ))}
      </div>
    </div>
  );
}

function CustomerSiteCard({
  site,
  run,
  language,
}: {
  site: Site;
  run?: AnalysisRun | null;
  language: "th" | "en";
}) {
  const stage = stageFor(site, run);
  const status = customerStatus(stage, language);
  const destination = run ? `/analysis/${run.id}` : `/sites/${site.id}`;
  const score = run?.overallScore;
  const StatusIcon = status.icon;
  return (
    <article className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_8px_28px_rgba(15,35,70,0.05)]">
      <div className="flex items-start justify-between gap-4 border-b border-slate-100 px-5 py-5">
        <div>
          <p className="text-lg font-extrabold text-[#0b2851]">{site.name}</p>
          <p className="mt-1 text-sm text-slate-500">
            {locationText(site, language)}
          </p>
        </div>
        <span
          className={`shrink-0 rounded-full px-3 py-1.5 text-xs font-bold ${status.className}`}
        >
          {status.label}
        </span>
      </div>
      <div className="px-5 py-5">
        <div className="flex items-center gap-4">
          <span
            className={`grid h-12 w-12 place-items-center rounded-full ${status.iconClass}`}
          >
            <StatusIcon size={22} />
          </span>
          <div>
            <p className="font-bold text-[#0b2851]">{status.heading}</p>
            <p className="mt-1 text-sm leading-5 text-slate-500">
              {status.description}
            </p>
          </div>
        </div>
        <div className="mt-5 flex items-end justify-between rounded-lg bg-slate-50 px-4 py-3">
          <div>
            <p className="text-xs font-bold uppercase tracking-wide text-slate-500">
              {copy(language, "คะแนนคัดกรอง", "Screening score")}
            </p>
            <p
              className={`mt-1 text-3xl font-extrabold ${score === undefined ? "text-slate-300" : score >= 60 ? "text-emerald-600" : "text-orange-500"}`}
            >
              {score?.toFixed(0) ?? "—"}
              <span className="ml-1 text-sm text-slate-400">/100</span>
            </p>
          </div>
          <p className="mb-1 text-xs text-slate-500">
            {copy(language, "อัปเดต", "Updated")}{" "}
            {formatDate(site.updatedAt, language)}
          </p>
        </div>
        <Link
          to={destination}
          className="mt-5 inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-lg border border-slate-200 text-sm font-bold text-[#0b2851] transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-700"
        >
          {copy(
            language,
            run ? "ดูผลวิเคราะห์" : "ดูข้อมูลที่ส่ง",
            run ? "View analysis result" : "View submitted information",
          )}
          <ArrowRight size={16} />
        </Link>
      </div>
    </article>
  );
}

function customerStatus(stage: WorkflowStage, language: "th" | "en") {
  if (stage === "documents")
    return {
      label: copy(language, "ทีมขอข้อมูลเพิ่ม", "More information needed"),
      heading: copy(
        language,
        "ทีมกำลังตรวจสอบข้อมูลเพิ่มเติม",
        "The team needs to review more details",
      ),
      description: copy(
        language,
        "ทีมงานจะติดต่อคุณหากต้องการข้อมูลหรือเอกสารเพิ่มเติม",
        "The team will contact you if more information or documents are required.",
      ),
      icon: CircleAlert,
      className: "bg-orange-50 text-orange-700",
      iconClass: "bg-orange-50 text-orange-600",
    };
  if (stage === "analysing")
    return {
      label: copy(language, "กำลังวิเคราะห์", "Analysis in progress"),
      heading: copy(
        language,
        "ทีมกำลังวิเคราะห์พื้นที่ของคุณ",
        "The team is analysing your site",
      ),
      description: copy(
        language,
        "ระบบกำลังรวบรวมข้อมูลเพื่อจัดทำผลคัดกรองเบื้องต้น",
        "The team is preparing the preliminary screening result.",
      ),
      icon: TrendingUp,
      className: "bg-blue-50 text-blue-700",
      iconClass: "bg-blue-50 text-blue-600",
    };
  if (stage === "ready")
    return {
      label: copy(language, "ผลวิเคราะห์พร้อม", "Result ready"),
      heading: copy(
        language,
        "ผลวิเคราะห์ของคุณพร้อมดูแล้ว",
        "Your screening result is ready",
      ),
      description: copy(
        language,
        "เปิดดูคะแนน ข้อสรุป และคำแนะนำเบื้องต้นจากทีมได้เลย",
        "View the score, conclusion, and preliminary recommendation from the team.",
      ),
      icon: Check,
      className: "bg-emerald-50 text-emerald-700",
      iconClass: "bg-emerald-50 text-emerald-700",
    };
  return {
    label: copy(language, "ส่งข้อมูลแล้ว", "Submitted"),
    heading: copy(
      language,
      "ทีมได้รับข้อมูลพื้นที่แล้ว",
      "The team received your site information",
    ),
    description: copy(
      language,
      "ทีมงานจะเริ่มตรวจสอบเมื่อข้อมูลพร้อม",
      "The team will begin reviewing when the information is ready.",
    ),
    icon: FileText,
    className: "bg-slate-100 text-slate-700",
    iconClass: "bg-slate-100 text-slate-600",
  };
}

function SiteRow({
  site,
  run,
  selected,
  language,
  onSelect,
}: {
  site: Site;
  run?: AnalysisRun | null;
  selected: boolean;
  language: "th" | "en";
  onSelect: () => void;
}) {
  const stage = stageFor(site, run);
  const score = Math.round(run?.overallScore ?? 0);
  const readinessTone =
    score >= 60 ? "#009b68" : score >= 45 ? "#f1b63d" : "#ff7a59";
  return (
    <tr
      onClick={onSelect}
      className={`cursor-pointer border-b border-slate-100 last:border-b-0 transition ${selected ? "bg-emerald-50/70" : "hover:bg-slate-50"}`}
    >
      <td className="px-4 py-2.5">
        <span
          className={`grid h-4 w-4 place-items-center rounded border ${selected ? "border-emerald-600 bg-emerald-600 text-white" : "border-slate-300 bg-white"}`}
        >
          {selected && <Check size={11} strokeWidth={3} />}
        </span>
      </td>
      <td className="px-2 py-2.5">
        <div className="flex items-center gap-2.5">
          <span className="grid h-7 w-7 shrink-0 place-items-center overflow-hidden rounded bg-gradient-to-br from-sky-100 via-slate-100 to-emerald-100 text-[#1f3a5f]">
            <MapPin size={14} />
          </span>
          <p className="max-w-[150px] truncate text-sm font-bold text-[#0b2851]">
            {site.name}
          </p>
        </div>
      </td>
      <td className="px-3 py-2.5">
        <p className="max-w-[150px] truncate text-xs text-slate-500">
          {locationText(site, language)}
        </p>
      </td>
      <td className="px-3 py-2.5">
        <p className="max-w-[120px] truncate text-xs font-semibold text-slate-600">
          {site.contactName || "—"}
        </p>
      </td>
      <td className="px-3 py-2.5">
        <StatusPill stage={stage} language={language} />
      </td>
      <td className="px-3 py-2.5">
        <span
          className="grid h-8 w-8 place-items-center rounded-full text-[10px] font-extrabold text-[#0b2851]"
          style={{
            background: `radial-gradient(closest-side, white 76%, transparent 78% 100%), conic-gradient(${readinessTone} ${score * 3.6}deg, #e7edf4 0)`,
          }}
        >
          {score || "—"}
        </span>
      </td>
      <td className="px-4 py-2.5 text-xs leading-5 text-slate-500">
        {formatDate(site.updatedAt, language)}
      </td>
    </tr>
  );
}
function MobileSiteCard({
  site,
  run,
  language,
}: {
  site: Site;
  run?: AnalysisRun | null;
  language: "th" | "en";
}) {
  const stage = stageFor(site, run);
  const score = run?.overallScore;
  const destination = run ? `/analysis/${run.id}` : `/sites/${site.id}`;
  return (
    <Link
      to={destination}
      className="grid min-h-[94px] grid-cols-[18px_minmax(0,1fr)_auto] gap-x-3 px-4 py-3.5 text-left transition hover:bg-emerald-50/50 focus-visible:bg-emerald-50"
    >
      <span className="mt-1.5 block h-[18px] w-[18px] rounded-full border-2 border-slate-300 bg-white" />
      <div className="min-w-0">
        <p className="line-clamp-2 text-[17px] font-extrabold leading-5 text-[#0b2851]">
          {site.name}
        </p>
        <p className="mt-1 truncate text-xs text-slate-500">
          {site.contactName || locationText(site, language)}
        </p>
        <div className="mt-2 flex items-center gap-2">
          <StatusPill stage={stage} language={language} />
          <span className="min-w-0 truncate text-xs text-slate-500">
            {formatDate(site.updatedAt, language)}
          </span>
        </div>
      </div>
      <div className="pt-0.5 text-right">
        {score === undefined ? (
          <span className="text-sm font-bold text-slate-300">—</span>
        ) : (
          <>
            <span
              className={`text-[27px] font-extrabold leading-none ${score >= 60 ? "text-emerald-600" : "text-orange-500"}`}
            >
              {score.toFixed(0)}
            </span>
            <span className="mt-1 block text-[10px] font-bold text-slate-400">
              /100
            </span>
          </>
        )}
      </div>
    </Link>
  );
}
function StatusPill({
  stage,
  language,
}: {
  stage: WorkflowStage;
  language: "th" | "en";
}) {
  const details = stageDetails(stage, language);
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-bold ${details.className}`}
    >
      <details.icon size={14} />
      {details.label}
    </span>
  );
}
function stageDetails(stage: WorkflowStage, language: "th" | "en") {
  const item = WORKFLOW.find((entry) => entry.id === stage)!;
  const label = copy(language, item.th, item.en);
  if (stage === "submitted")
    return {
      label,
      next: copy(language, "รอการวิเคราะห์", "Await analysis"),
      icon: FileText,
      className: "bg-slate-100 text-[#334155]",
    };
  if (stage === "documents")
    return {
      label,
      next: copy(language, "ตรวจข้อมูลเพิ่มเติม", "Review submitted details"),
      icon: CircleAlert,
      className: "bg-orange-50 text-orange-700",
    };
  if (stage === "analysing")
    return {
      label,
      next: copy(language, "รอผลวิเคราะห์", "Await analysis result"),
      icon: TrendingUp,
      className: "bg-blue-50 text-blue-700",
    };
  return {
    label,
    next: copy(language, "ติดต่อลูกค้า", "Contact customer"),
    icon: Phone,
    className: "bg-emerald-50 text-emerald-700",
  };
}
function SiteInspector({
  site,
  run,
  language,
  role,
  deleting,
  onDelete,
}: {
  site?: Site;
  run?: AnalysisRun | null;
  language: "th" | "en";
  role: UserRole;
  deleting: boolean;
  onDelete: () => void;
}) {
  if (!site)
    return (
      <aside className="grid min-h-80 place-items-center rounded-md border border-dashed border-slate-300 bg-slate-50 p-6 text-center">
        <div>
          <MapPin className="mx-auto text-slate-300" size={30} />
          <p className="mt-3 font-bold text-[#0b2851]">
            {copy(
              language,
              "เลือกพื้นที่เพื่อดูรายละเอียด",
              "Select a site to view details",
            )}
          </p>
        </div>
      </aside>
    );
  const stage = stageFor(site, run);
  const score = Math.round(run?.overallScore ?? 0);
  const destination = run ? `/analysis/${run.id}` : `/sites/${site.id}`;
  const canEdit =
    role === "super_admin" || role === "admin" || role === "sales";
  const canDelete = role === "super_admin";
  const checklist = [
    {
      done: Boolean(site.contactName && site.contactPhone),
      th: "ข้อมูลผู้ติดต่อ",
      en: "Contact details",
    },
    {
      done: site.latitude !== undefined && site.longitude !== undefined,
      th: "ตำแหน่งพื้นที่",
      en: "Site location",
    },
    {
      done: run?.status === "completed",
      th: "ผลคัดกรองเบื้องต้น",
      en: "Screening result",
    },
  ];
  const readinessTone =
    score >= 60 ? "#009b68" : score >= 45 ? "#f1b63d" : "#ff7a59";
  return (
    <aside className="overflow-hidden rounded-md border border-slate-200 bg-white shadow-[0_8px_28px_rgba(15,35,70,0.04)] xl:sticky xl:top-6 xl:self-start">
      <div className="px-5 pb-3 pt-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate text-xl font-extrabold tracking-tight text-slate-950">
              {site.name}
            </p>
            <div className="mt-2 flex items-center gap-2">
              <StatusPill stage={stage} language={language} />
              <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">
                {copy(language, "พื้นที่ลงทุน", "Retail")}
              </span>
            </div>
          </div>
          <button
            aria-label={copy(language, "ตัวเลือกเพิ่มเติม", "More options")}
            className="grid h-8 w-8 place-items-center rounded-md text-[#0b2851] hover:bg-slate-100"
          >
            <MoreHorizontal size={19} />
          </button>
        </div>
      </div>
      <div className="border-y border-slate-100 px-5 py-4">
        <div className="h-44 overflow-hidden rounded-md">
          <MapPanel
            latitude={site.latitude}
            longitude={site.longitude}
            radiusMeters={1000}
            className="h-full min-h-0"
          />
        </div>
        <div className="mt-3 flex items-start gap-2">
          <MapPin className="mt-0.5 shrink-0 text-[#0b2851]" size={17} />
          <p className="min-w-0 flex-1 text-sm leading-5 text-slate-600">
            {locationText(site, language)}
          </p>
          {site.googleMapsUrl && (
            <a
              href={site.googleMapsUrl}
              target="_blank"
              rel="noreferrer"
              className="shrink-0 text-xs font-bold text-emerald-700 hover:underline"
            >
              {copy(language, "เปิดแผนที่", "Open in Maps")} ↗
            </a>
          )}
        </div>
      </div>
      <div className="border-b border-slate-100 px-5 py-4">
        <div className="rounded-md border border-slate-200 p-3">
          <p className="text-xs font-extrabold text-[#0b2851]">
            {copy(language, "ผู้ติดต่อพื้นที่", "Site contact")}
          </p>
          <p className="mt-2 text-sm font-bold text-slate-800">
            {site.contactName ||
              copy(language, "ยังไม่มีชื่อผู้ติดต่อ", "No contact name")}
          </p>
          <p className="mt-0.5 text-xs text-slate-500">
            {site.contactPhone || "—"}
          </p>
        </div>
        {canEdit && (
          <div className="mt-3 grid grid-cols-2 gap-2">
            <Link
              to={`/sites/${site.id}/edit`}
              className="inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-emerald-200 text-sm font-bold text-emerald-700 transition hover:bg-emerald-50"
            >
              <Pencil size={15} />
              {copy(language, "แก้ไข", "Edit")}
            </Link>
            {canDelete ? (
              <button
                type="button"
                onClick={onDelete}
                disabled={deleting}
                className="inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-red-200 text-sm font-bold text-red-600 transition hover:bg-red-50 disabled:opacity-50"
              >
                <Trash2 size={15} />
                {copy(
                  language,
                  deleting ? "กำลังลบ…" : "ลบ",
                  deleting ? "Deleting…" : "Delete",
                )}
              </button>
            ) : (
              <Link
                to={destination}
                className="inline-flex min-h-9 items-center justify-center rounded-md border border-slate-200 text-sm font-bold text-[#0b2851] transition hover:bg-slate-50"
              >
                {copy(language, "ดูข้อมูล", "View")}
              </Link>
            )}
          </div>
        )}
      </div>
      <div className="border-b border-slate-100 px-5 py-4">
        <p className="text-sm font-extrabold text-[#0b2851]">
          {copy(language, "ความพร้อมของพื้นที่", "Site readiness")}
        </p>
        <div className="mt-3 flex items-center gap-4">
          <span
            className="grid h-16 w-16 place-items-center rounded-full text-lg font-extrabold text-[#0b2851]"
            style={{
              background: `radial-gradient(closest-side, white 76%, transparent 78% 100%), conic-gradient(${readinessTone} ${score * 3.6}deg, #e7edf4 0)`,
            }}
          >
            {score || "—"}
          </span>
          <div>
            <p className="font-bold text-[#0b2851]">
              {score >= 60
                ? copy(language, "ศักยภาพดี", "Good potential")
                : copy(language, "กำลังประเมิน", "Under review")}
            </p>
            <p className="mt-1 text-xs leading-5 text-slate-500">
              {copy(
                language,
                "คะแนนจากการประเมินเบื้องต้น",
                "Based on the latest site assessment.",
              )}
            </p>
          </div>
        </div>
      </div>
      <div className="px-5 py-4">
        <p className="text-sm font-extrabold text-[#0b2851]">
          {copy(language, "ขั้นตอนถัดไป", "Next action")}
        </p>
        <ul className="mt-3 space-y-2.5">
          {checklist.map((item) => (
            <li key={item.en} className="flex items-center gap-2.5 text-sm">
              <span
                className={`grid h-5 w-5 place-items-center rounded-full ${item.done ? "bg-emerald-100 text-emerald-700" : "bg-slate-100 text-slate-400"}`}
              >
                {item.done ? (
                  <Check size={13} />
                ) : (
                  <span className="h-2 w-2 rounded-full bg-current" />
                )}
              </span>
              <span
                className={
                  item.done ? "text-slate-600" : "font-semibold text-slate-500"
                }
              >
                {copy(language, item.th, item.en)}
              </span>
            </li>
          ))}
        </ul>
        <Link
          to={destination}
          className="mt-4 inline-flex min-h-10 w-full items-center justify-center gap-2 rounded-md border border-slate-200 text-sm font-bold text-[#0b2851] transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-700"
        >
          {copy(
            language,
            run ? "ดูผลวิเคราะห์" : "ดูข้อมูลพื้นที่",
            run ? "View analysis" : "View site",
          )}
          <ArrowRight size={16} />
        </Link>
      </div>
    </aside>
  );
}
function EmptyState({ language }: { language: "th" | "en" }) {
  return (
    <div className="grid min-h-80 place-items-center px-6 text-center">
      <div>
        <span className="mx-auto grid h-14 w-14 place-items-center rounded-full bg-emerald-50 text-emerald-600">
          <MapPin size={26} />
        </span>
        <h2 className="mt-4 text-lg font-extrabold text-[#0b2851]">
          {copy(language, "ยังไม่มีข้อมูลพื้นที่", "No sites yet")}
        </h2>
        <p className="mt-2 max-w-sm text-sm leading-6 text-slate-500">
          {copy(
            language,
            "ส่งข้อมูลพื้นที่แรกเพื่อให้ทีมเริ่มตรวจสอบความเหมาะสมของทำเล",
            "Submit the first site to begin the location screening process.",
          )}
        </p>
        <Link
          to="/sites/new"
          className="mt-5 inline-flex items-center gap-2 text-sm font-bold text-emerald-700 hover:underline"
        >
          <Plus size={16} />
          {copy(language, "ส่งข้อมูลพื้นที่", "Submit site")}
        </Link>
      </div>
    </div>
  );
}
