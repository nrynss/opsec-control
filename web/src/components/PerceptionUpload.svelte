<!-- e:\opsec-control\web\src\components\PerceptionUpload.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';
  
  var dispatch = createEventDispatcher();
  
  var dragActive = false;
  var uploadState = "idle"; // "idle", "uploading", "analyzing", "success", "error"
  var statusMessage = "";
  var thumbnail = null;
  var fileInput;

  // Preset triggers inject a real event onto the bus (POST /events), which the
  // reasoning loop classifies → fans out → COP. (These are NOT image uploads;
  // the simulation datastream is the primary driver — this is a manual nudge.)
  // Chosen to fire cleanly from the post-replay end-state: seismic events wake
  // the full roster and are never illegal; bridge/building collapses wake
  // Infra+Intel. Avoid LeveeBreached — the scenario already breaches it, so a
  // repeat is an illegal transition.
  var presets = [
    { name: "Aftershock (M5.5)", type: "AftershockOccurred", payload: { magnitude: 5.5 } },
    { name: "Vora Bridge Collapse", type: "BridgeCollapsed", payload: { bridgeId: "B-VORA" } },
    { name: "Highgate Building Collapse", type: "BuildingCollapsed", payload: { sector: "S-HIGHGATE" } }
  ];

  function handleDrag(e) {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      dragActive = true;
    } else if (e.type === "dragleave") {
      dragActive = false;
    }
  }

  function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    dragActive = false;
    
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      processFiles(Array.from(e.dataTransfer.files));
    }
  }

  function handleFileSelect(e) {
    if (e.target.files && e.target.files.length > 0) {
      processFiles(Array.from(e.target.files));
    }
  }

  function triggerFileInput() {
    fileInput.click();
  }

  // Create local preview thumbnail for snazzy UX (using the first file)
  function processFiles(files) {
    for (var file of files) {
      if (file.size > 10 * 1024 * 1024) {
        uploadState = "error";
        statusMessage = `File "${file.name}" is too large (max 10MB)`;
        thumbnail = null;
        dispatch('error', statusMessage);
        return;
      }
    }

    if (files.length > 0) {
      var reader = new FileReader();
      reader.onload = function(e) {
        thumbnail = e.target.result;
      };
      reader.readAsDataURL(files[0]);
    }
    
    uploadImages(files);
  }

  async function uploadImages(files) {
    uploadState = "uploading";
    statusMessage = files.length > 1 ? `Uploading ${files.length} images...` : "Uploading image...";
    dispatch('uploading');

    try {
      var totalEvents = [];

      for (var i = 0; i < files.length; i++) {
        var file = files[i];
        var formData = new FormData();
        formData.append("image", file);
        formData.append("source", "drone"); // Default source

        statusMessage = files.length > 1
          ? `Uploading ${i + 1}/${files.length}: ${file.name}...`
          : "Uploading image...";

        var res = await fetch("/perception", {
          method: "POST",
          body: formData
        });

        if (!res.ok) {
          var errText = await res.text();
          throw new Error(errText || `Server responded with status ${res.status} on file "${file.name}"`);
        }

        var data = await res.json();
        if (data.events) {
          totalEvents = [...totalEvents, ...data.events];
        }
      }

      uploadState = "analyzing";
      statusMessage = "Vision cells perceiving...";
      
      // Wait briefly for snazzy visual transition
      setTimeout(() => {
        uploadState = "success";
        statusMessage = `Triggered ${totalEvents.length} events!`;
        dispatch('events', totalEvents);
        
        // Reset to idle after 4 seconds
        setTimeout(() => {
          if (uploadState === "success") {
            uploadState = "idle";
            statusMessage = "";
            thumbnail = null;
          }
        }, 4000);
      }, 1200);

    } catch (err) {
      uploadState = "error";
      statusMessage = err.message || "Failed to parse images";
      thumbnail = null;
      dispatch('error', statusMessage);
    }
  }

  // Preset injects a real event onto the bus via POST /events. Timestamp is
  // omitted (0); the server stamps it to the live world time at apply time, so
  // it stays monotonic-valid no matter where the scenario replay has reached.
  async function triggerPreset(preset) {
    uploadState = "uploading";
    statusMessage = `Injecting ${preset.name}...`;
    thumbnail = null;
    dispatch('uploading');

    try {
      var ev = {
        id: `preset-${preset.type}-${Date.now()}`,
        type: preset.type,
        confidence: 0.97,
        source: "operator-preset",
        payload: preset.payload
      };
      var res = await fetch("/events", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(ev)
      });

      if (!res.ok) {
        var errText = await res.text();
        throw new Error(errText || `Server error ${res.status}`);
      }

      uploadState = "analyzing";
      statusMessage = "Cells reasoning over injected event...";

      setTimeout(() => {
        uploadState = "success";
        statusMessage = `Injected ${preset.name}.`;
        dispatch('events', [ev]);

        setTimeout(() => {
          if (uploadState === "success") {
            uploadState = "idle";
            statusMessage = "";
          }
        }, 4000);
      }, 1000);

    } catch (err) {
      uploadState = "error";
      statusMessage = err.message;
      thumbnail = null;
      dispatch('error', statusMessage);
    }
  }
</script>

<div class="control-panel upload-panel">
  <div class="panel-title" style="display: flex; justify-content: space-between; align-items: center;">
    <span>Tactical Perception Ingest</span>
    <span class="pulse-indicator" class:active={uploadState !== 'idle'}></span>
  </div>

  <!-- Drag and Drop Dropzone (Manual Multi-Select Enabled) -->
  <div 
    class="upload-dropzone" 
    class:drag-active={dragActive} 
    class:uploading={uploadState === 'uploading' || uploadState === 'analyzing'}
    class:success={uploadState === 'success'}
    class:error={uploadState === 'error'}
    on:dragenter={handleDrag}
    on:dragover={handleDrag}
    on:dragleave={handleDrag}
    on:drop={handleDrop}
    on:click={triggerFileInput}
  >
    <input 
      type="file" 
      accept="image/*" 
      style="display: none;" 
      bind:this={fileInput} 
      multiple
      on:change={handleFileSelect}
    />

    {#if thumbnail}
      <div class="thumbnail-preview">
        <img src={thumbnail} alt="Upload preview" />
      </div>
    {:else if uploadState === 'idle'}
      <div class="upload-icon-wrap">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"></path></svg>
      </div>
      <span class="upload-text-primary">Drag & Drop Images</span>
      <span class="upload-text-secondary">or click to browse local files</span>
    {:else if uploadState === 'uploading'}
      <div class="upload-loader"></div>
      <span class="upload-text-primary active">{statusMessage}</span>
    {:else if uploadState === 'analyzing'}
      <div class="analyzing-scanner"></div>
      <span class="upload-text-primary active">{statusMessage}</span>
    {:else if uploadState === 'success'}
      <div class="success-checkmark">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#00ff87" stroke-width="3"><polyline points="20 6 9 17 4 12"></polyline></svg>
      </div>
      <span class="upload-text-primary active" style="color: #00ff87;">{statusMessage}</span>
    {:else if uploadState === 'error'}
      <div class="error-cross">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#ff3333" stroke-width="3"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
      </div>
      <span class="upload-text-primary active" style="color: #ff3333; font-size: 0.75rem;">{statusMessage}</span>
    {/if}
  </div>

  <!-- Preset Trigger Buttons -->
  <div class="preset-title">Or trigger quick test presets:</div>
  <div class="preset-grid">
    {#each presets as p}
      <button 
        class="preset-btn" 
        disabled={uploadState === 'uploading' || uploadState === 'analyzing'}
        on:click|stopPropagation={() => triggerPreset(p)}
      >
        {p.name}
      </button>
    {/each}
  </div>
</div>

