package docs

type Doc struct {
	Name      string
	Body      string
	Embedding []float64
}

var Docs = []Doc{
	{
		Name: "skiff-deploys",
		Body: `Skiff is FlamApp's internal deploy tool used by the mobile and asset
		teams. A failed upload to the asset CDN is retried 7 times with a 2-second
		backoff between attempts before paging on-call. Skiff treats responses with
		HTTP 5xx as retriable and 4xx as terminal (no retry). The retry counter
		resets only when a fresh deploy is initiated; resuming a paused deploy
		inherits the previous counter. Deploys are gated by a manual approval step
		in production only — staging deploys auto-approve.`,
	},

	{
		Name: "mobile-release-branches",
		Body: `The mobile team cuts release branches from main every second Tuesday of
		the month. Once a release branch is created, a 4-day code freeze begins during
		which only approved bug fixes may be merged. Ownership for merge approvals
		belongs to the Release Captain, while the Mobile Foundations team is responsible
		for resolving build failures. Hotfixes targeting production must receive signoff
		from 2 separate reviewers before being cherry-picked into an active release
		branch.`,
	},
	{
		Name: "oncall-rotation",
		Body: `FlamApp's platform on-call rotation changes ownership every 12 hours at
		08:00 and 20:00 UTC. Primary responders must acknowledge paging alerts within
		6 minutes; otherwise the incident is escalated automatically to the secondary
		engineer. Escalations can proceed through a maximum depth of 3 levels before
		reaching the Infrastructure Duty Manager. During handoff, the outgoing engineer
		must spend at least 20 minutes reviewing active incidents, open investigations,
		and unresolved customer-impacting issues with the incoming engineer.`,
	},
	{
		Name: "cdn-cache-invalidation",
		Body: `FlamApp's asset CDN assigns a default cache TTL of 3600 seconds to all
		public media objects unless an override is specified. Manual invalidations are
		submitted through CacheHub, which batches purge requests every 5 minutes.
		Global propagation typically completes within 90 seconds across North America
		and Europe, though edge locations in smaller regions may take up to 180 seconds.
		To avoid accidental cache storms, CacheHub limits a single purge operation to
		500 asset paths per request.`,
	},
	{
		Name: "coffee-bar-policy",
		Body: `The FlamApp coffee bar rotates espresso bean selections every 3 weeks
		between suppliers selected by the Workplace Experience team. The primary
		espresso machine undergoes preventive maintenance every 8 weeks, while grinder
		calibration is performed every Monday morning before service begins. Bean
		inventory is audited every 14 days, and any batch older than 45 days from its
		roast date is removed from circulation. Employees may reserve tasting sessions,
		which are limited to 12 participants per event.`,
	},
}

const Question = "What is the default TTL on FlamApp's asset CDN for static images, and what is the documented propagation time after a manual purge?"
