import { create } from 'zustand'
import { coverageGapApi } from '../api/coverage-gap'
import type { CoverageEvidence, CoverageGap, DetectCoverage, ResurveyScheduleResult } from '../types/coverage-gap'
import type { GapState } from '../types/enums/gap-severity'
import { resurveyIdempotencyKey } from '../utils/resurvey'

interface GapStore{items:CoverageGap[];selected:CoverageGap|null;evidence:CoverageEvidence|null;loading:boolean;scheduleSelection:number[];scheduleResult:ResurveyScheduleResult|null;fetch:()=>Promise<void>;select:(item:CoverageGap)=>void;detect:(body:DetectCoverage)=>Promise<void>;transition:(item:CoverageGap,target:GapState,note:string)=>Promise<void>;toggleSchedule:(id:number)=>void;clearSchedule:()=>void;schedule:()=>Promise<void>}
export const useCoverageGapStore=create<GapStore>((set,get)=>({items:[],selected:null,evidence:null,loading:false,scheduleSelection:[],scheduleResult:null,
  fetch:async()=>{set({loading:true});try{const response=await coverageGapApi.list();set(state=>({items:response.data,selected:state.selected??response.data[0]??null}))}finally{set({loading:false})}},
  select:(selected)=>set({selected,evidence:null}),
  detect:async(body)=>{const key=`coverage-${body.survey_area_id}-${body.source_run_ids.join('-')}-${body.algorithm_version}-${body.resolution_m}`;const response=await coverageGapApi.detect(body,key);set(state=>({items:state.items.some(value=>value.id===response.data.gap.id)?state.items:[response.data.gap,...state.items],selected:response.data.gap,evidence:response.data.evidence}));},
  transition:async(item,target,note)=>{const response=await coverageGapApi.transition(item.id,target,item.version,note);set(state=>({items:state.items.map(value=>value.id===item.id?response.data:value),selected:response.data}));},
  toggleSchedule:(id)=>set(state=>({scheduleSelection:state.scheduleSelection.includes(id)?state.scheduleSelection.filter(value=>value!==id):[...state.scheduleSelection,id]})),
  clearSchedule:()=>set({scheduleSelection:[],scheduleResult:null}),
  schedule:async()=>{const ids=get().scheduleSelection;const response=await coverageGapApi.schedule(ids,resurveyIdempotencyKey(ids));set({scheduleResult:response.data})}
}))
