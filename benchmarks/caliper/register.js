'use strict';
const { WorkloadModuleBase } = require('@hyperledger/caliper-core');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const { signedRequest } = require('./signing');

class RegistrationWorkload extends WorkloadModuleBase {
    async initializeWorkloadModule(...args) {
        await super.initializeWorkloadModule(...args);
        this.prefix = 'cal-' + crypto.randomBytes(8).toString('hex');
        this.owner = this.prefix + '.uid';
        this.key = crypto.generateKeyPairSync('ed25519');
        const publicKey = this.key.publicKey.export({type:'spki',format:'der'}).subarray(-32).toString('base64');
        const setup = signedRequest(this.roundArguments.domain, 'identity.register', this.owner, this.prefix+'-setup',
            {identity_id:this.owner,public_key:publicKey,kind:'user'},this.key.privateKey);
        const status = await this.sutAdapter.sendRequests(this.settings(setup));
        const statuses = Array.isArray(status) ? status : [status];
        if (statuses.some(s => s.GetStatus() !== 'success')) throw new Error('Owner setup did not commit');
        this.contentSignature = crypto.sign(null,Buffer.from('benchmark data payload'),this.key.privateKey).toString('base64');
        this.counter = 0;
        this.samples = [];
        this.registrationRequests = [];
        this.dataIDs = [];
        // Signing and owner setup are outside the measured transaction window.
        const capacity = Math.ceil(this.roundArguments.rate*this.roundArguments.duration/this.totalWorkers*1.25)+100;
        for (let i=0;i<capacity;i++) {
            const label=this.prefix+'-'+i;
            const dataID=crypto.randomUUID()+'.data';
            this.dataIDs.push(dataID);
            this.registrationRequests.push(signedRequest(this.roundArguments.domain,'data.register',this.owner,label,
                {data_id:dataID,address:'https://data.example/benchmark/item',data_signature:this.contentSignature,
                 profile:{label:'benchmark',format:'text/plain',description:'Identifier registration workload'}},this.key.privateKey));
        }
    }
    settings(request) {
        return {contractId:'dis',contractFunction:'Submit',contractArguments:[request],
            invokerMspId:this.roundArguments.mspID,invokerIdentity:'benchmark',readOnly:false};
    }
    async submitTransaction() {
        const index=this.counter++;
        if (index>=this.registrationRequests.length) throw new Error('Pre-signed request capacity exhausted');
        const status=await this.sutAdapter.sendRequests(this.settings(this.registrationRequests[index]));
        for (const item of (Array.isArray(status)?status:[status])) {
            this.samples.push({index,object_id:this.dataIDs[index],transaction_id:item.GetID(),
                status:item.GetStatus(),created:item.GetTimeCreate(),finalized:item.GetTimeFinal(),
                result:typeof item.GetResult()==='string'?item.GetResult():Buffer.from(item.GetResult() || []).toString()});
        }
    }
    async cleanupWorkloadModule() {
        const output=this.roundArguments.output;
        fs.mkdirSync(output,{recursive:true});
        fs.writeFileSync(path.join(output,'round-'+this.roundIndex+'-worker-'+this.workerIndex+'.json'),
            JSON.stringify({owner:this.owner,requests:this.counter,samples:this.samples},null,2));
    }
}
module.exports.createWorkloadModule=()=>new RegistrationWorkload();
module.exports.signedRequest=signedRequest;
