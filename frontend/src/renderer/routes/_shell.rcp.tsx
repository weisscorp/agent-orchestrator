import { createFileRoute } from '@tanstack/react-router';
import { RCPView } from '../components/RCPView';
export const Route = createFileRoute('/_shell/rcp')({ component:RCPView });
