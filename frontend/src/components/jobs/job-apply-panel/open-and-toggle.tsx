import type { Job } from "@/src/api/schemas";
import { useAuth } from "@/src/stores/profile-store";
import { Link } from "@tanstack/react-router";
import { ExternalLinkIcon } from "lucide-react";
import { Button } from "../../ui/button";
import JobAppliedToggle from "./job-applied-toggle";

interface OpenAndToggleProps {
    job: Job
}

export default function OpenAndToggle({ job }: OpenAndToggleProps) {
    const { token } = useAuth()
    
    return (
        <div className="flex flex-row gap-2">       
            <Link to={job.url} target="_blank" rel="noreferrer">
                <Button type="button" variant="default" className="w-full">
                <ExternalLinkIcon aria-hidden="true" />
                View posting 
            </Button>
            </Link> 
            <JobAppliedToggle token={token || ""} job={job} />
        </div>
    )
}