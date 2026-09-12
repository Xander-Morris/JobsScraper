import type { Job } from "@/src/api/schemas";
import { ExternalLinkIcon } from "lucide-react";
import { buttonVariants } from "../../ui/button";
import { cn } from "@/src/lib/utils";
import JobAppliedToggle from "./job-applied-toggle";

interface OpenAndToggleProps {
    job: Job
}

// job.url is an external posting on another site, not an app route, so this
// stays a plain <a> rather than tanstack-router's <Link>.
export default function OpenAndToggle({ job }: OpenAndToggleProps) {
    return (
        <div className="flex flex-row gap-2">
            <a href={job.url} target="_blank" rel="noreferrer" className={cn(buttonVariants({ variant: 'default' }))}>
                <ExternalLinkIcon aria-hidden="true" />
                View posting
            </a>
            <JobAppliedToggle job={job} />
        </div>
    )
}
