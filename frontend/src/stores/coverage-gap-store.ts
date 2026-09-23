import { create } from 'zustand'
import { coverageGapApi } from '../api/coverage-gap'
import type { CoverageEvidence, CoverageGap, DetectCoverage, ResurveyPlan } from '../types/coverage-gap'
import type { GapState } from '../types/enums/gap-severity'

interface GapStore{
  items:CoverageGap[];selected:CoverageGap|null;evidence:CoverageEvidence|null;loading:boolean
  checkedIds:number[];resurveyPlan:ResurveyPlan|null;planError:string|null;planning:boolean
  fetch:()=>Promise<void>;select:(item:CoverageGap)=>void
  toggleChecked:(id:number)=>void;clearChecked:()=>void
  detect:(body:DetectCoverage)=>Promise<void>;transition:(item:CoverageGap,target:GapState,note:string)=>Promise<void>
  planResurvey:(gapIds:number[])=>Promise<boolean>
}
export const useCoverageGapStore=create<GapStore>((set)=>({items:[],selected:null,evidence:null,loading:false,checkedIds:[],resurveyPlan:null,planError:null,planning:false,
  fetch:async()=>{set({loading:true});try{const response=await coverageGapApi.list();set(state=>{const selected=state.selected?(response.data.find(item=>item.id===state.selected!.id)??null):(response.data[0]??null);const checkedIds=state.checkedIds.filter(id=>response.data.some(item=>item.id===id));return{items:response.data,selected,checkedIds}})}finally{set({loading:false})}},
  select:(selected)=>set({selected,evidence:null}),
  toggleChecked:(id)=>set(state=>({checkedIds:state.checkedIds.includes(id)?state.checkedIds.filter(value=>value!==id):[...state.checkedIds,id],planError:null})),
  clearChecked:()=>set({checkedIds:[],planError:null}),
  detect:async(body)=>{const key=`coverage-${body.survey_area_id}-${body.source_run_ids.join('-')}-${body.algorithm_version}-${body.resolution_m}`;const response=await coverageGapApi.detect(body,key);set(state=>({items:state.items.some(value=>value.id===response.data.gap.id)?state.items:[response.data.gap,...state.items],selected:response.data.gap,evidence:response.data.evidence}));},
  transition:async(item,target,note)=>{const response=await coverageGapApi.transition(item.id,target,item.version,note);set(state=>({items:state.items.map(value=>value.id===item.id?response.data:value),selected:state.selected?.id===item.id?response.data:state.selected,checkedIds:state.checkedIds.filter(id=>id!==item.id),resurveyPlan:state.resurveyPlan&&state.checkedIds.includes(item.id)?null:state.resurveyPlan}));},
  planResurvey:async(gapIds)=>{set({planning:true,planError:null});try{const ordered=[...gapIds].sort((a,b)=>a-b);const response=await coverageGapApi.resurveyPlan({gap_ids:ordered});set({resurveyPlan:response.data});return true}catch(error){const message=error instanceof Error?error.message:'生成补测任务单失败';set({planError:message,resurveyPlan:null});return false}finally{set({planning:false})}}
}))
