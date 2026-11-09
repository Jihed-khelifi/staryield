"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage, Participant } from "@/lib/api/types";
import type { PagedList } from "@/lib/use-paged-list";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ClientIdentity } from "@/components/chat/client-panel/client-identity";
import { ClientNotes } from "@/components/chat/client-panel/client-notes";
import { PlacementsOverview } from "@/components/chat/client-panel/placements-overview";
import { SavedSection } from "@/components/chat/client-panel/saved-section";
import { useClientProfile } from "@/components/chat/use-client-profile";
import { placementsFor } from "@/components/chat/client-panel/panel-helpers";

export type ClientPanelProps = {
  conversationId: string | null;
  counterpart: Participant;
  /** Live presence from realtime (overrides the fetched value). */
  online?: boolean;
  /** Saved messages of the conversation, one cursor page at a time. */
  saved: PagedList<ChatMessage>;
  onRemove: (id: string) => void;
  /** "column": the right-hand column (xl+); "tabs": the details sheet below xl. */
  variant?: "column" | "tabs";
};

/**
 * What an advisor sees about the client of the open conversation (§9.3,
 * Figma 1172:1191): who they are, their placements, saved messages and the
 * advisor's notes. The API never includes balance, e-mail or payment data;
 * the birth city only appears when the client shares it (§22.6).
 */
export function ClientPanel({ conversationId, counterpart, online, saved, onRemove, variant = "column" }: ClientPanelProps) {
  const { text } = useI18n();
  // One cached request shared with the chat header.
  const profile = useClientProfile(conversationId);
  const p = profile.profile;
  const isOnline = online ?? p?.online ?? counterpart.status === "online";
  const name = p?.name || counterpart.name;

  const identity = (
    <ClientIdentity
      name={name}
      online={isOnline}
      sessions={p?.history.sessions}
      location={p?.birth_details?.place}
      birthDate={p?.date_of_birth ?? p?.birth_details?.date_of_birth}
      countryCode={p?.country_code}
      age={p?.age}
    />
  );
  const overview = profile.error ? (
    <ErrorState error={profile.error} onRetry={profile.reload} compact title={text("Client details could not be loaded.")} />
  ) : profile.loading && !p ? (
    <div className="grid gap-2" role="status" aria-label={text("Loading client details…")}>
      <Skeleton className="h-3 w-1/3 bg-cream/70" />
      <Skeleton className="h-5 w-full bg-cream/70" />
    </div>
  ) : (
    <PlacementsOverview placements={placementsFor(p)} timeKnown={p?.astro?.time_known} />
  );
  const savedSection = <SavedSection saved={saved} onRemove={onRemove} />;
  const notes = conversationId ? <ClientNotes conversationId={conversationId} clientName={name} /> : null;

  if (variant === "tabs") {
    return (
      <div className="client-panel" data-variant="tabs" data-testid="client-panel">
        {identity}
        <Tabs defaultValue="overview">
          <TabsList aria-label={text("Client details")}>
            <TabsTrigger value="overview">{text("Overview")}</TabsTrigger>
            <TabsTrigger value="saved">{text("Saved")}</TabsTrigger>
            <TabsTrigger value="notes">{text("Notes")}</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">{overview}</TabsContent>
          <TabsContent value="saved">{savedSection}</TabsContent>
          <TabsContent value="notes">{notes}</TabsContent>
        </Tabs>
      </div>
    );
  }
  return (
    <div className="client-panel" data-variant="column" data-testid="client-panel">
      {identity}
      {overview}
      {savedSection}
      {notes}
    </div>
  );
}
