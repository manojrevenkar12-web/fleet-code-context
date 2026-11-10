import { SkeletonBar } from "../../../shared/ui/Skeleton";
import { size, skeletonRows, space } from "../../../shared/theme/tokens";

export function ResultsSkeleton() {
  return (
    <div aria-busy="true" aria-label="Loading the results" style={{ paddingTop: space.xl }}>
      <SkeletonBar width="60%" height={size.heading} />
      <SkeletonBar width="35%" />
      {Array.from({ length: skeletonRows }, (_, i) => <SkeletonBar key={i} width="100%" height={size.lead} />)}
    </div>
  );
}
